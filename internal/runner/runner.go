package runner

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/79E/auto-openai-account/internal/domain"
	"github.com/79E/auto-openai-account/internal/legacy"
	"github.com/79E/auto-openai-account/internal/storage"
)

type Runner struct {
	store  *storage.Store
	mu     sync.Mutex
	cancel context.CancelFunc
	subs   map[int64]map[chan domain.RuntimeLog]struct{}
	active map[string]activeLogContext
}

type activeLogContext struct {
	JobID     int64
	MailboxID int64
	Email     string
	Proxy     string
}

var roundRobinCounter uint64

func New(store *storage.Store) *Runner {
	r := &Runner{store: store, subs: map[int64]map[chan domain.RuntimeLog]struct{}{}, active: map[string]activeLogContext{}}
	legacy.LogHook = r.handleLegacyLog
	return r
}

func (r *Runner) Start(count int) (domain.RegisterJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	running, err := r.store.RunningJobExists()
	if err != nil {
		return domain.RegisterJob{}, err
	}
	if running || r.cancel != nil {
		return domain.RegisterJob{}, fmt.Errorf("register job is already running")
	}
	available, err := r.store.CountMailboxesByStatus(domain.MailboxStatusNew)
	if err != nil {
		return domain.RegisterJob{}, err
	}
	if count < 1 {
		return domain.RegisterJob{}, fmt.Errorf("count must be greater than 0")
	}
	if count > available {
		return domain.RegisterJob{}, fmt.Errorf("count exceeds new mailbox count: %d", available)
	}
	items, err := r.store.PickNewMailboxes(count)
	if err != nil {
		return domain.RegisterJob{}, err
	}
	job, err := r.store.CreateJob(count, items)
	if err != nil {
		return domain.RegisterJob{}, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	go r.run(ctx, job.ID, items)
	return job, nil
}

func (r *Runner) StartLogin(ids []int64) (domain.RegisterJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	running, err := r.store.RunningJobExists()
	if err != nil {
		return domain.RegisterJob{}, err
	}
	if running || r.cancel != nil {
		return domain.RegisterJob{}, fmt.Errorf("job is already running")
	}
	if len(ids) == 0 {
		return domain.RegisterJob{}, fmt.Errorf("mailbox_ids is required")
	}
	items, err := r.store.PickMailboxesByIDs(ids)
	if err != nil {
		return domain.RegisterJob{}, err
	}
	if len(items) == 0 {
		return domain.RegisterJob{}, fmt.Errorf("no mailboxes found")
	}
	job, err := r.store.CreateTypedJob(domain.JobTypeLogin, len(items), items)
	if err != nil {
		return domain.RegisterJob{}, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	go r.runLogin(ctx, job.ID, items)
	return job, nil
}

func (r *Runner) Stop(jobID int64) error {
	r.mu.Lock()
	if r.cancel != nil {
		r.cancel()
	}
	r.mu.Unlock()
	r.log(domain.RuntimeLog{JobID: jobID, Level: "info", Step: "stopped", Message: "任务已手动结束"})
	return r.store.StopJob(jobID)
}

func (r *Runner) Subscribe(jobID int64) (<-chan domain.RuntimeLog, func()) {
	ch := make(chan domain.RuntimeLog, 32)
	r.mu.Lock()
	if r.subs[jobID] == nil {
		r.subs[jobID] = map[chan domain.RuntimeLog]struct{}{}
	}
	r.subs[jobID][ch] = struct{}{}
	r.mu.Unlock()
	return ch, func() {
		r.mu.Lock()
		delete(r.subs[jobID], ch)
		close(ch)
		r.mu.Unlock()
	}
}

func (r *Runner) run(ctx context.Context, jobID int64, items []domain.Mailbox) {
	defer func() {
		r.mu.Lock()
		r.cancel = nil
		r.mu.Unlock()
	}()
	settings, err := r.store.LoadSettings()
	if err != nil {
		_ = r.store.RecalculateJob(jobID, domain.JobStatusFailed)
		return
	}
	concurrency := settings.RegisterConcurrency
	jobs := make(chan domain.Mailbox)
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for mailbox := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				r.runOne(ctx, jobID, mailbox, settings)
			}
		}(i)
	}
	for _, mailbox := range items {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			_ = r.store.RecalculateJob(jobID, domain.JobStatusStopped)
			return
		case jobs <- mailbox:
		}
	}
	close(jobs)
	wg.Wait()
	status := domain.JobStatusFinished
	if ctx.Err() != nil {
		status = domain.JobStatusStopped
	}
	_ = r.store.RecalculateJob(jobID, status)
}

func (r *Runner) runLogin(ctx context.Context, jobID int64, items []domain.Mailbox) {
	defer func() {
		r.mu.Lock()
		r.cancel = nil
		r.mu.Unlock()
	}()
	settings, err := r.store.LoadSettings()
	if err != nil {
		_ = r.store.RecalculateJob(jobID, domain.JobStatusFailed)
		return
	}
	concurrency := settings.RegisterConcurrency
	jobs := make(chan domain.Mailbox)
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for mailbox := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				r.runLoginOne(ctx, jobID, mailbox, settings)
			}
		}()
	}
	for _, mailbox := range items {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			_ = r.store.RecalculateJob(jobID, domain.JobStatusStopped)
			return
		case jobs <- mailbox:
		}
	}
	close(jobs)
	wg.Wait()
	status := domain.JobStatusFinished
	if ctx.Err() != nil {
		status = domain.JobStatusStopped
	}
	_ = r.store.RecalculateJob(jobID, status)
}

func (r *Runner) runOne(ctx context.Context, jobID int64, mailbox domain.Mailbox, settings domain.Settings) {
	started := time.Now()
	proxy := pickProxy(settings)
	_ = r.store.StartJobItem(jobID, mailbox.ID)
	r.setActive(mailbox.Email, activeLogContext{JobID: jobID, MailboxID: mailbox.ID, Email: mailbox.Email, Proxy: proxy})
	defer r.clearActive(mailbox.Email)
	legacyMailbox := legacy.MailboxFromDomain(mailbox)
	legacySettings := legacy.SettingsFromDomain(settings, proxy)
	provider := legacy.OTPProvider{Settings: legacySettings, Mailbox: legacyMailbox, Since: started}
	registerPass := legacyPasswordForSettings(settings)
	result, err := legacy.RegisterOne(ctx, legacy.RegisterInput{Mailbox: legacyMailbox, Settings: legacySettings, RegisterPass: registerPass, OTPFetcher: provider.Fetch})
	duration := time.Since(started)
	if ctx.Err() != nil {
		_ = r.store.UpdateJobItem(jobID, mailbox.ID, "failed", "手动结束任务", duration)
		_ = r.store.RecalculateJob(jobID, domain.JobStatusStopped)
		return
	}
	if err != nil {
		message := legacy.ExplainError(err.Error())
		_ = r.store.MarkMailboxAbnormal(mailbox.ID, message)
		_ = r.store.UpdateJobItem(jobID, mailbox.ID, "failed", message, duration)
		_ = r.store.RecalculateJob(jobID, "")
		r.log(domain.RuntimeLog{JobID: jobID, MailboxID: mailbox.ID, Email: mailbox.Email, Level: "error", Step: "failed", Message: message})
		return
	}
	_ = r.store.MarkMailboxRegistered(mailbox.ID, result.Password, legacy.CompactTokenJSON(result.TokenPayload))
	_ = r.store.UpdateJobItem(jobID, mailbox.ID, "success", "", duration)
	_ = r.store.RecalculateJob(jobID, "")
	r.log(domain.RuntimeLog{JobID: jobID, MailboxID: mailbox.ID, Email: mailbox.Email, Level: "info", Step: "complete", StepIndex: 8, StepTotal: 8, Message: "注册流程完成"})
}

func (r *Runner) LoginMailbox(mailbox domain.Mailbox, settings domain.Settings) error {
	if err := r.store.MarkMailboxLogining(mailbox.ID); err != nil {
		return err
	}
	go func() {
		started := time.Now()
		proxy := pickProxy(settings)
		r.setActive(mailbox.Email, activeLogContext{MailboxID: mailbox.ID, Email: mailbox.Email, Proxy: proxy})
		defer r.clearActive(mailbox.Email)
		legacyMailbox := legacy.MailboxFromDomain(mailbox)
		legacySettings := legacy.SettingsFromDomain(settings, proxy)
		provider := legacy.OTPProvider{Settings: legacySettings, Mailbox: legacyMailbox, Since: started}
		tokens, err := legacy.LoginOne(context.Background(), legacyMailbox, legacySettings, provider.Fetch)
		if err != nil {
			_ = r.store.MarkMailboxLoginResult(mailbox.ID, "", legacy.ExplainError(err.Error()))
			r.log(domain.RuntimeLog{MailboxID: mailbox.ID, Email: mailbox.Email, Level: "error", Step: "login_failed", Message: legacy.ExplainError(err.Error())})
			return
		}
		_ = r.store.MarkMailboxLoginResult(mailbox.ID, legacy.CompactTokenJSON(tokens), "")
		r.log(domain.RuntimeLog{MailboxID: mailbox.ID, Email: mailbox.Email, Level: "info", Step: "login_complete", Message: "登录换 token 流程完成"})
	}()
	return nil
}

func (r *Runner) runLoginOne(ctx context.Context, jobID int64, mailbox domain.Mailbox, settings domain.Settings) {
	started := time.Now()
	proxy := pickProxy(settings)
	_ = r.store.StartJobItem(jobID, mailbox.ID)
	_ = r.store.MarkMailboxLogining(mailbox.ID)
	r.setActive(mailbox.Email, activeLogContext{JobID: jobID, MailboxID: mailbox.ID, Email: mailbox.Email, Proxy: proxy})
	defer r.clearActive(mailbox.Email)
	legacyMailbox := legacy.MailboxFromDomain(mailbox)
	legacySettings := legacy.SettingsFromDomain(settings, proxy)
	provider := legacy.OTPProvider{Settings: legacySettings, Mailbox: legacyMailbox, Since: started}
	tokens, err := legacy.LoginOne(ctx, legacyMailbox, legacySettings, provider.Fetch)
	duration := time.Since(started)
	if ctx.Err() != nil {
		_ = r.store.UpdateJobItem(jobID, mailbox.ID, "failed", "手动结束任务", duration)
		_ = r.store.RecalculateJob(jobID, domain.JobStatusStopped)
		return
	}
	if err != nil {
		message := legacy.ExplainError(err.Error())
		_ = r.store.MarkMailboxLoginResult(mailbox.ID, "", message)
		_ = r.store.UpdateJobItem(jobID, mailbox.ID, "failed", message, duration)
		_ = r.store.RecalculateJob(jobID, "")
		r.log(domain.RuntimeLog{JobID: jobID, MailboxID: mailbox.ID, Email: mailbox.Email, Level: "error", Step: "login_failed", Message: message})
		return
	}
	_ = r.store.MarkMailboxLoginResult(mailbox.ID, legacy.CompactTokenJSON(tokens), "")
	_ = r.store.UpdateJobItem(jobID, mailbox.ID, "success", "", duration)
	_ = r.store.RecalculateJob(jobID, "")
	r.log(domain.RuntimeLog{JobID: jobID, MailboxID: mailbox.ID, Email: mailbox.Email, Level: "info", Step: "login_complete", Message: "登录换 token 流程完成"})
}

func (r *Runner) log(entry domain.RuntimeLog) {
	entry, err := r.store.AddLog(entry)
	if err != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for ch := range r.subs[entry.JobID] {
		select {
		case ch <- entry:
		default:
		}
	}
}

func (r *Runner) handleLegacyLog(email, message string) {
	if email == "" {
		return
	}
	r.mu.Lock()
	ctx, ok := r.active[strings.ToLower(email)]
	r.mu.Unlock()
	if !ok {
		return
	}
	stepIndex, stepTotal, step := parseLegacyStep(message)
	if step != "" {
		_ = r.store.MarkMailboxStep(ctx.MailboxID, domain.MailboxStatusRegistering, step, stepIndex, stepTotal, ctx.Proxy)
	}
	r.log(domain.RuntimeLog{JobID: ctx.JobID, MailboxID: ctx.MailboxID, Email: ctx.Email, Level: "info", Step: step, StepIndex: stepIndex, StepTotal: stepTotal, Message: message})

}

func (r *Runner) setActive(email string, ctx activeLogContext) {
	r.mu.Lock()
	r.active[strings.ToLower(email)] = ctx
	r.mu.Unlock()
}

func (r *Runner) clearActive(email string) {
	r.mu.Lock()
	delete(r.active, strings.ToLower(email))
	r.mu.Unlock()
}

func parseLegacyStep(message string) (int, int, string) {
	if !strings.HasPrefix(message, "步骤 ") {
		return 0, 0, ""
	}
	fields := strings.Fields(message)
	if len(fields) < 2 {
		return 0, 0, ""
	}
	parts := strings.Split(fields[1], "/")
	if len(parts) != 2 {
		return 0, 0, ""
	}
	idx, _ := strconv.Atoi(parts[0])
	total, _ := strconv.Atoi(parts[1])
	step := "step_" + parts[0]
	if len(fields) > 2 {
		step = strings.ToLower(strings.ReplaceAll(fields[2], "-", "_"))
	}
	return idx, total, step
}

func legacyPasswordForSettings(settings domain.Settings) string {
	if settings.PasswordMode == "fixed" && settings.FixedPassword != "" {
		return settings.FixedPassword
	}
	return ""
}

func pickProxy(settings domain.Settings) string {
	if len(settings.Proxies) == 0 {
		return ""
	}
	if settings.ProxyMode == "single" {
		return settings.Proxies[0]
	}
	if settings.ProxyMode == "round_robin" {
		idx := atomic.AddUint64(&roundRobinCounter, 1)
		return settings.Proxies[int(idx-1)%len(settings.Proxies)]
	}
	return settings.Proxies[rand.Intn(len(settings.Proxies))]
}
