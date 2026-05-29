package runner

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/79E/auto-openai-account/internal/codex"
	"github.com/79E/auto-openai-account/internal/domain"
	"github.com/79E/auto-openai-account/internal/proxypool"
	"github.com/79E/auto-openai-account/internal/legacy"
	"github.com/79E/auto-openai-account/internal/smsbiz"
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

type taskProxyChoice struct {
	UseLocal   bool
	GroupName  string
	Group      domain.ProxyGroup
}

type proxyRuntime struct {
	mu         sync.Mutex
	proxies    []string
	mode       string
	currentIdx int
	current    string
	locked     bool
}

func New(store *storage.Store) *Runner {
	r := &Runner{store: store, subs: map[int64]map[chan domain.RuntimeLog]struct{}{}, active: map[string]activeLogContext{}}
	legacy.LogHook = r.handleLegacyLog
	return r
}

func (r *Runner) Start(count int, flow string, smsConfigName string, proxyGroupName string) (domain.RegisterJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	flow, err := normalizeRegisterFlow(flow)
	if err != nil {
		return domain.RegisterJob{}, err
	}
	running, err := r.store.RunningJobExists()
	if err != nil {
		return domain.RegisterJob{}, err
	}
	if running || r.cancel != nil {
		return domain.RegisterJob{}, fmt.Errorf("register job is already running")
	}
	settings, err := r.store.LoadSettings()
	if err != nil {
		return domain.RegisterJob{}, err
	}
	if flow == domain.JobTypeRegisterCodex {
		if _, err := requireSMSConfig(settings, smsConfigName); err != nil {
			return domain.RegisterJob{}, err
		}
	}
	proxyChoice, err := resolveTaskProxyChoice(settings, proxyGroupName)
	if err != nil {
		return domain.RegisterJob{}, err
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
	job, err := r.store.CreateTypedJob(flow, count, items)
	if err != nil {
		return domain.RegisterJob{}, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	go r.runRegister(ctx, job.ID, items, flow, smsConfigName, proxyChoice)
	return job, nil
}

func (r *Runner) StartLogin(ids []int64, flow string, smsConfigName string, proxyGroupName string) (domain.RegisterJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	flow, err := normalizeLoginFlow(flow)
	if err != nil {
		return domain.RegisterJob{}, err
	}
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
	settings, err := r.store.LoadSettings()
	if err != nil {
		return domain.RegisterJob{}, err
	}
	if flow == domain.JobTypeCodexLogin {
		if _, err := requireSMSConfig(settings, smsConfigName); err != nil {
			return domain.RegisterJob{}, err
		}
	}
	proxyChoice, err := resolveTaskProxyChoice(settings, proxyGroupName)
	if err != nil {
		return domain.RegisterJob{}, err
	}
	for _, item := range items {
		if mailboxLoginPassword(item) == "" {
			return domain.RegisterJob{}, fmt.Errorf("mailbox %s does not have a password for login", item.Email)
		}
	}
	job, err := r.store.CreateTypedJob(flow, len(items), items)
	if err != nil {
		return domain.RegisterJob{}, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	go r.runLogin(ctx, job.ID, items, flow, smsConfigName, proxyChoice)
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

func (r *Runner) runRegister(ctx context.Context, jobID int64, items []domain.Mailbox, flow string, smsConfigName string, proxyChoice taskProxyChoice) {
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
				selector := newProxyRuntime(proxyChoice)
				r.runRegisterOne(ctx, jobID, mailbox, settings, flow, smsConfigName, selector)
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

func (r *Runner) runLogin(ctx context.Context, jobID int64, items []domain.Mailbox, flow string, smsConfigName string, proxyChoice taskProxyChoice) {
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
				selector := newProxyRuntime(proxyChoice)
				if flow == domain.JobTypeCodexLogin {
					r.runCodexLoginOne(ctx, jobID, mailbox, settings, smsConfigName, selector)
					continue
				}
				r.runLoginOne(ctx, jobID, mailbox, settings, selector)
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

func (r *Runner) runRegisterOne(ctx context.Context, jobID int64, mailbox domain.Mailbox, settings domain.Settings, flow string, smsConfigName string, proxySelector *proxyRuntime) {
	started := time.Now()
	proxy, err := proxySelector.Start(ctx)
	_ = r.store.StartJobItem(jobID, mailbox.ID)
	if err != nil {
		message := legacy.ExplainError(err.Error())
		_ = r.store.MarkMailboxAbnormal(mailbox.ID, message)
		_ = r.store.UpdateJobItem(jobID, mailbox.ID, "failed", message, time.Since(started))
		_ = r.store.RecalculateJob(jobID, "")
		r.log(domain.RuntimeLog{JobID: jobID, MailboxID: mailbox.ID, Email: mailbox.Email, Level: "error", Step: "proxy_failed", Message: message})
		return
	}
	r.setActive(mailbox.Email, activeLogContext{JobID: jobID, MailboxID: mailbox.ID, Email: mailbox.Email, Proxy: proxy})
	defer r.clearActive(mailbox.Email)
	legacyMailbox := legacy.MailboxFromDomain(mailbox)
	legacySettings := legacy.SettingsFromDomain(settings, proxy, proxySelector)
	provider := legacy.OTPProvider{Settings: legacySettings, Mailbox: legacyMailbox, Since: started}
	registerPass := legacyPasswordForSettings(settings)
	skipTokenLogin := flow == domain.JobTypeRegister
	result, err := legacy.RegisterOne(ctx, legacy.RegisterInput{Mailbox: legacyMailbox, Settings: legacySettings, ProxyController: proxySelector, RegisterPass: registerPass, OTPFetcher: provider.Fetch, SkipTokenLogin: skipTokenLogin})
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
	if flow == domain.JobTypeRegisterCodex {
		updated := mailbox
		updated.RegisterPassword = result.Password
		r.runCodexLoginAfterStarted(ctx, jobID, updated, settings, proxySelector, smsConfigName, started, "register_codex")
		return
	}
	_ = r.store.UpdateJobItem(jobID, mailbox.ID, "success", "", duration)
	_ = r.store.RecalculateJob(jobID, "")
	message := "注册流程完成"
	if flow == domain.JobTypeRegisterLogin {
		message = "注册并普通登录流程完成"
	}
	r.log(domain.RuntimeLog{JobID: jobID, MailboxID: mailbox.ID, Email: mailbox.Email, Level: "info", Step: "complete", StepIndex: 8, StepTotal: 8, Message: message})
}

func (r *Runner) LoginMailbox(mailbox domain.Mailbox, settings domain.Settings) error {
	if err := r.store.MarkMailboxLogining(mailbox.ID); err != nil {
		return err
	}
	go func() {
		started := time.Now()
		selector := newProxyRuntime(taskProxyChoice{UseLocal: true})
		proxy, err := selector.Start(context.Background())
		if err != nil {
			_ = r.store.MarkMailboxLoginResult(mailbox.ID, "", legacy.ExplainError(err.Error()))
			return
		}
		r.setActive(mailbox.Email, activeLogContext{MailboxID: mailbox.ID, Email: mailbox.Email, Proxy: proxy})
		defer r.clearActive(mailbox.Email)
		legacyMailbox := legacy.MailboxFromDomain(mailbox)
		legacySettings := legacy.SettingsFromDomain(settings, proxy, selector)
		provider := legacy.OTPProvider{Settings: legacySettings, Mailbox: legacyMailbox, Since: started}
		tokens, err := legacy.LoginOne(context.Background(), legacyMailbox, legacySettings, provider.Fetch, selector)
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

func (r *Runner) runLoginOne(ctx context.Context, jobID int64, mailbox domain.Mailbox, settings domain.Settings, proxySelector *proxyRuntime) {
	started := time.Now()
	_ = r.store.StartJobItem(jobID, mailbox.ID)
	_ = r.store.MarkMailboxLogining(mailbox.ID)
	proxy, err := proxySelector.Start(ctx)
	if err != nil {
		message := legacy.ExplainError(err.Error())
		_ = r.store.MarkMailboxLoginResult(mailbox.ID, "", message)
		_ = r.store.UpdateJobItem(jobID, mailbox.ID, "failed", message, time.Since(started))
		_ = r.store.RecalculateJob(jobID, "")
		r.log(domain.RuntimeLog{JobID: jobID, MailboxID: mailbox.ID, Email: mailbox.Email, Level: "error", Step: "proxy_failed", Message: message})
		return
	}
	r.setActive(mailbox.Email, activeLogContext{JobID: jobID, MailboxID: mailbox.ID, Email: mailbox.Email, Proxy: proxy})
	defer r.clearActive(mailbox.Email)
	legacyMailbox := legacy.MailboxFromDomain(mailbox)
	legacySettings := legacy.SettingsFromDomain(settings, proxy, proxySelector)
	provider := legacy.OTPProvider{Settings: legacySettings, Mailbox: legacyMailbox, Since: started}
	tokens, err := legacy.LoginOne(ctx, legacyMailbox, legacySettings, provider.Fetch, proxySelector)
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

func (r *Runner) runCodexLoginOne(ctx context.Context, jobID int64, mailbox domain.Mailbox, settings domain.Settings, smsConfigName string, proxySelector *proxyRuntime) {
	started := time.Now()
	_ = r.store.StartJobItem(jobID, mailbox.ID)
	_ = r.store.MarkMailboxLogining(mailbox.ID)
	proxy, err := proxySelector.Start(ctx)
	if err != nil {
		message := legacy.ExplainError(err.Error())
		_ = r.store.MarkMailboxLoginResult(mailbox.ID, "", message)
		_ = r.store.UpdateJobItem(jobID, mailbox.ID, "failed", message, time.Since(started))
		_ = r.store.RecalculateJob(jobID, "")
		r.log(domain.RuntimeLog{JobID: jobID, MailboxID: mailbox.ID, Email: mailbox.Email, Level: "error", Step: "proxy_failed", Message: message})
		return
	}
	r.setActive(mailbox.Email, activeLogContext{JobID: jobID, MailboxID: mailbox.ID, Email: mailbox.Email, Proxy: proxy})
	defer r.clearActive(mailbox.Email)
	r.log(domain.RuntimeLog{JobID: jobID, MailboxID: mailbox.ID, Email: mailbox.Email, Level: "info", Step: "codex_start", StepIndex: 1, StepTotal: 8, Message: "Codex 授权登录流程开始"})
	r.runCodexLoginAfterStarted(ctx, jobID, mailbox, settings, proxySelector, smsConfigName, started, "codex")
}

func (r *Runner) runCodexLoginAfterStarted(ctx context.Context, jobID int64, mailbox domain.Mailbox, settings domain.Settings, proxySelector *proxyRuntime, smsConfigName string, started time.Time, prefix string) {
	proxy := proxySelector.CurrentProxy()
	duration := time.Since(started)
	if ctx.Err() != nil {
		_ = r.store.UpdateJobItem(jobID, mailbox.ID, "failed", "手动结束任务", duration)
		_ = r.store.RecalculateJob(jobID, domain.JobStatusStopped)
		return
	}
	smsConfig, err := requireSMSConfig(settings, smsConfigName)
	if err != nil {
		r.failCodexJobItem(jobID, mailbox, prefix, err.Error(), duration)
		return
	}
	provider, err := smsbiz.NewProvider(smsbiz.Config{
		Platform:  smsConfig.Platform,
		APIKey:    smsConfig.APIKey,
		ServiceID: smsConfig.ServiceID,
		CountryID: smsConfig.CountryID,
		MaxPrice:  smsConfig.MaxPrice,
	})
	if err != nil {
		r.failCodexJobItem(jobID, mailbox, prefix, fmt.Sprintf("短信平台初始化失败: %v", err), duration)
		return
	}
	defer provider.Close()
	legacyMailbox := legacy.MailboxFromDomain(mailbox)
	legacySettings := legacy.SettingsFromDomain(settings, proxy, proxySelector)
	otpProvider := legacy.OTPProvider{Settings: legacySettings, Mailbox: legacyMailbox, Since: started}
	canFetchEmailOTP := legacyMailbox.CanFetchEmailOTP(legacySettings)
	if !canFetchEmailOTP {
		r.log(domain.RuntimeLog{JobID: jobID, MailboxID: mailbox.ID, Email: mailbox.Email, Level: "info", Step: prefix + "_email_otp_unavailable", Message: "邮箱认证必要数据不完整，Codex 登录将仅尝试密码登录"})
	}
	progressCh := make(chan codex.LoginProgress, 32)
	progressDone := make(chan struct{})
	go func() {
		defer close(progressDone)
		for progress := range progressCh {
			step := string(progress.Step)
			if step == "" {
				step = prefix + "_progress"
			}
			_ = r.store.MarkMailboxStep(mailbox.ID, domain.MailboxStatusLogining, step, progress.StepIndex, progress.StepTotal, proxy)
			r.log(domain.RuntimeLog{JobID: jobID, MailboxID: mailbox.ID, Email: mailbox.Email, Level: "info", Step: step, StepIndex: progress.StepIndex, StepTotal: progress.StepTotal, Message: progress.Message})
		}
	}()
	loginOpts := codex.LoginOptions{
		Email:                    mailbox.Email,
		Password:                 mailboxLoginPassword(mailbox),
		Proxy:                    proxy,
		ProxyController:          proxySelector,
		SMSProvider:              &codexSMSProvider{provider: provider, config: smsConfig},
		OTPFetcher:               func(ctx context.Context) (string, error) {
			if !canFetchEmailOTP {
				return "", fmt.Errorf("mailbox is missing required email auth data for otp fallback")
			}
			return otpProvider.Fetch(ctx)
		},
		ProgressChan:             progressCh,
		MaxPhoneAttempts:         3,
		PasswordVerifyRetries:    codexPasswordVerifyRetries(prefix),
		PasswordVerifyRetryDelay: 10 * time.Second,
	}
	result, err := codex.LoginWithCodex(ctx, loginOpts)
	close(progressCh)
	<-progressDone
	duration = time.Since(started)
	if ctx.Err() != nil {
		_ = r.store.UpdateJobItem(jobID, mailbox.ID, "failed", "手动结束任务", duration)
		_ = r.store.RecalculateJob(jobID, domain.JobStatusStopped)
		return
	}
	if err != nil {
		r.failCodexJobItem(jobID, mailbox, prefix, err.Error(), duration)
		return
	}
	if result.PhoneNumber != "" {
		_ = r.store.UpdateMailboxPhoneNumber(mailbox.ID, result.PhoneNumber)
	}
	_ = r.store.MarkMailboxLoginResult(mailbox.ID, result.TokenJSON, "")
	_ = r.store.UpdateJobItem(jobID, mailbox.ID, "success", "", duration)
	_ = r.store.RecalculateJob(jobID, "")
	r.log(domain.RuntimeLog{JobID: jobID, MailboxID: mailbox.ID, Email: mailbox.Email, Level: "info", Step: "codex_complete", StepIndex: 8, StepTotal: 8, Message: "Codex 授权登录流程完成"})
}

func generateRandomPassword() string {
	length := 16
	upper := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	lower := "abcdefghijklmnopqrstuvwxyz"
	digits := "0123456789"
	special := "!@#$%"
	all := upper + lower + digits + special
	value := []byte{
		upper[rand.Intn(len(upper))],
		lower[rand.Intn(len(lower))],
		digits[rand.Intn(len(digits))],
		special[rand.Intn(len(special))],
	}
	for len(value) < length {
		value = append(value, all[rand.Intn(len(all))])
	}
	for i := range value {
		j := rand.Intn(i + 1)
		value[i], value[j] = value[j], value[i]
	}
	return string(value)
}

func normalizeRegisterFlow(flow string) (string, error) {
	switch strings.TrimSpace(flow) {
	case "":
		return domain.JobTypeRegisterLogin, nil
	case domain.JobTypeRegister, domain.JobTypeRegisterLogin, domain.JobTypeRegisterCodex:
		return strings.TrimSpace(flow), nil
	default:
		return "", fmt.Errorf("unsupported register flow: %s", flow)
	}
}

func normalizeLoginFlow(flow string) (string, error) {
	switch strings.TrimSpace(flow) {
	case "":
		return domain.JobTypeLogin, nil
	case domain.JobTypeLogin, domain.JobTypeCodexLogin:
		return strings.TrimSpace(flow), nil
	default:
		return "", fmt.Errorf("unsupported login flow: %s", flow)
	}
}

func requireSMSConfig(settings domain.Settings, name string) (domain.SMSConfig, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.SMSConfig{}, fmt.Errorf("sms_config_name is required for codex flow")
	}
	for _, cfg := range settings.SMSConfigs {
		if strings.EqualFold(strings.TrimSpace(cfg.Name), name) {
			if strings.TrimSpace(cfg.APIKey) == "" {
				return domain.SMSConfig{}, fmt.Errorf("sms config %q missing api_key", name)
			}
			return cfg, nil
		}
	}
	return domain.SMSConfig{}, fmt.Errorf("sms config %q not found", name)
}

func mailboxLoginPassword(mailbox domain.Mailbox) string {
	if password := strings.TrimSpace(mailbox.RegisterPassword); password != "" {
		return password
	}
	return strings.TrimSpace(mailbox.Password)
}

func codexPasswordVerifyRetries(prefix string) int {
	if prefix == "register_codex" {
		return 3
	}
	return 1
}

func (r *Runner) failCodexJobItem(jobID int64, mailbox domain.Mailbox, prefix string, message string, duration time.Duration) {
	if prefix == "" {
		prefix = "codex"
	}
	message = legacy.ExplainError(message)
	_ = r.store.MarkMailboxLoginResult(mailbox.ID, "", message)
	_ = r.store.UpdateJobItem(jobID, mailbox.ID, "failed", message, duration)
	_ = r.store.RecalculateJob(jobID, "")
	r.log(domain.RuntimeLog{JobID: jobID, MailboxID: mailbox.ID, Email: mailbox.Email, Level: "error", Step: prefix + "_failed", Message: message})
}

type codexSMSProvider struct {
	provider smsbiz.Provider
	config   domain.SMSConfig
}

func (p *codexSMSProvider) GetNumber(ctx context.Context) (*codex.SMSActivation, error) {
	activation, err := p.provider.GetNumber(ctx, p.config.ServiceID, p.config.CountryID, p.config.MaxPrice)
	if err != nil {
		return nil, err
	}
	return &codex.SMSActivation{
		ID:               activation.ActivationID,
		PhoneNumber:      activation.PhoneNumber,
		CountryPhoneCode: activation.CountryPhoneCode,
	}, nil
}

func (p *codexSMSProvider) PollCode(ctx context.Context, activationID string) (string, error) {
	return smsbiz.PollForCode(ctx, p.provider, activationID, 150*time.Second, 5*time.Second)
}

func (p *codexSMSProvider) Complete(ctx context.Context, activationID string) error {
	return p.provider.SetStatus(ctx, activationID, 6)
}

func (p *codexSMSProvider) Cancel(ctx context.Context, activationID string) error {
	return p.provider.SetStatus(ctx, activationID, 8)
}

func (r *Runner) log(entry domain.RuntimeLog) {
	entry.Message = semanticRuntimeMessage(entry)
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
	r.log(domain.RuntimeLog{JobID: ctx.JobID, MailboxID: ctx.MailboxID, Email: ctx.Email, Level: "info", Step: step, StepIndex: stepIndex, StepTotal: stepTotal, Message: semanticUILogMessage(message)})

}

func semanticRuntimeMessage(entry domain.RuntimeLog) string {
	message := strings.TrimSpace(entry.Message)
	if message == "" || entry.Level != "error" || strings.Contains(message, "原始错误：") {
		return message
	}
	return legacy.ExplainError(message)
}

func semanticUILogMessage(message string) string {
	base := uiLogMessage(message)
	details := safeLogDetails(message)
	if details == "" || strings.Contains(base, details) {
		return base
	}
	return base + "（" + details + "）"
}

func uiLogMessage(message string) string {
	message = strings.TrimSpace(message)
	switch {
	case strings.HasPrefix(message, "步骤 1/8"):
		return "正在初始化注册会话"
	case strings.HasPrefix(message, "步骤 2/8"):
		return "正在提交注册密码"
	case strings.HasPrefix(message, "步骤 3/8"):
		return "正在请求发送邮箱验证码"
	case strings.HasPrefix(message, "步骤 4/8"):
		return "正在等待并读取邮箱验证码"
	case strings.HasPrefix(message, "步骤 5/8"):
		return "已读取验证码，正在提交校验"
	case strings.HasPrefix(message, "步骤 6/8"):
		return "验证码通过，正在创建账号资料"
	case strings.HasPrefix(message, "步骤 7/8"):
		return "账号已创建，正在登录并换取 token"
	case strings.HasPrefix(message, "步骤 8/8"):
		return "注册完成，token 已获取"
	case strings.HasPrefix(message, "注册流程开始"):
		return "注册流程开始"
	case strings.HasPrefix(message, "注册流程完成"):
		return "注册流程完成"
	case strings.HasPrefix(message, "登录换 token 流程开始"):
		return "登录换 token 流程开始"
	case strings.Contains(message, "提交邮箱请求失败"):
		return "提交邮箱失败，正在停止当前流程"
	case strings.Contains(message, "提交邮箱") || strings.Contains(message, "重新提交邮箱"):
		return "正在提交邮箱并确认登录方式"
	case strings.Contains(message, "发送登录验证码失败"):
		return "发送登录验证码失败，正在尝试密码校验"
	case strings.Contains(message, "发送登录验证码"):
		return "正在发送登录验证码"
	case strings.Contains(message, "已获取登录验证码"):
		return "已读取登录验证码，正在提交校验"
	case strings.Contains(message, "登录验证码校验失败"):
		return "登录验证码校验失败，正在尝试密码校验"
	case strings.Contains(message, "登录验证码校验通过"):
		return "登录验证码校验通过"
	case strings.Contains(message, "构建 password_verify"):
		return "正在准备密码校验"
	case strings.Contains(message, "提交密码校验"):
		return "正在提交密码校验"
	case strings.Contains(message, "密码校验请求失败"):
		return "密码校验请求失败"
	case strings.Contains(message, "密码校验通过"):
		return "密码校验通过，正在完成授权"
	case strings.Contains(message, "密码校验返回"):
		return "已收到密码校验结果"
	case strings.Contains(message, "开始轮询邮箱验证码"):
		return "正在连接邮箱读取验证码"
	case strings.Contains(message, "邮箱验证码轮询第"):
		return "正在检查邮箱验证码"
	case strings.Contains(message, "邮箱验证码获取成功"):
		return "已从邮箱读取验证码"
	case strings.Contains(message, "邮箱验证码轮询失败"):
		return "本次读取邮箱验证码失败，稍后重试"
	case strings.Contains(message, "本次未找到验证码"):
		return "暂未找到验证码，等待后继续检查"
	case strings.Contains(message, "邮箱验证码超时"):
		return "读取邮箱验证码超时"
	case strings.Contains(message, "连接 IMAP"):
		return "正在连接邮箱服务器"
	case strings.Contains(message, "IMAP 登录认证"):
		return "正在认证邮箱"
	case strings.Contains(message, "IMAP 选择 INBOX") || strings.Contains(message, "IMAP 搜索全部邮件"):
		return "正在搜索收件箱邮件"
	case strings.Contains(message, "INBOX 没有邮件"):
		return "收件箱暂无邮件"
	case strings.Contains(message, "准备检查最近") || strings.Contains(message, "读取邮件"):
		return "正在检查最近邮件"
	case strings.Contains(message, "跳过："):
		return "已跳过一封不匹配的邮件"
	case strings.Contains(message, "可见正文未匹配到"):
		return "邮件中暂未匹配到验证码"
	case strings.Contains(message, "可见正文匹配到"):
		return "邮件中已匹配到验证码"
	case strings.Contains(message, "access token 刷新成功"):
		return "邮箱访问 token 已刷新"
	case strings.HasPrefix(message, "登录换 token 流程完成"):
		return "登录换 token 流程完成"
	default:
		return stripLogDetails(message)
	}
}

func stripLogDetails(message string) string {
	if idx := strings.Index(message, ": "); idx >= 0 && idx+2 < len(message) {
		prefix := message[:idx]
		if strings.Contains(prefix, "登录换 token") {
			message = message[idx+2:]
		}
	}
	for _, marker := range []string{" status=", " err=", " code=", " endpoint=", " ids=", " id=", " device_id=", " page_type=", " continue_url=", " password_len=", " token=", " context=", " timeout=", " attempt=", " location="} {
		if idx := strings.Index(message, marker); idx >= 0 {
			return strings.TrimSpace(message[:idx])
		}
	}
	return message
}

func safeLogDetails(message string) string {
	fields := []string{}
	for _, key := range []string{"status", "page_type", "passwordless_disabled", "attempt", "timeout", "poll", "imap", "auth", "endpoint", "password_len", "sentinel_token_len", "token_len", "location"} {
		if value := logDetailValue(message, key); value != "" {
			fields = append(fields, key+"="+value)
		}
	}
	if errCode := responseErrorCode(message); errCode != "" {
		fields = append(fields, "error_code="+errCode)
	}
	return strings.Join(fields, "，")
}

func logDetailValue(message, key string) string {
	marker := key + "="
	idx := strings.Index(message, marker)
	if idx < 0 {
		return ""
	}
	value := message[idx+len(marker):]
	if value == "" {
		return ""
	}
	if value[0] == '"' {
		end := strings.Index(value[1:], "\"")
		if end >= 0 {
			return value[:end+2]
		}
	}
	for i, r := range value {
		if r == ' ' || r == ',' || r == '，' || r == ')' || r == '）' {
			return value[:i]
		}
	}
	return value
}

func responseErrorCode(message string) string {
	marker := `"code":"`
	idx := strings.Index(message, marker)
	if idx < 0 {
		return ""
	}
	value := message[idx+len(marker):]
	end := strings.Index(value, `"`)
	if end < 0 {
		return ""
	}
	return value[:end]
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

func resolveTaskProxyChoice(settings domain.Settings, proxyGroupName string) (taskProxyChoice, error) {
	if strings.TrimSpace(proxyGroupName) == "" {
		return taskProxyChoice{UseLocal: true}, nil
	}
	group, ok := domain.FindProxyGroup(settings.ProxyGroups, proxyGroupName)
	if !ok {
		return taskProxyChoice{}, fmt.Errorf("proxy group %q not found", strings.TrimSpace(proxyGroupName))
	}
	if len(group.Proxies) == 0 {
		return taskProxyChoice{}, fmt.Errorf("proxy group %q has no proxies", group.Name)
	}
	return taskProxyChoice{GroupName: group.Name, Group: group}, nil
}

func newProxyRuntime(choice taskProxyChoice) *proxyRuntime {
	mode := "local"
	proxies := []string{}
	if !choice.UseLocal {
		mode = choice.Group.Mode
		proxies = append([]string(nil), choice.Group.Proxies...)
	}
	return &proxyRuntime{proxies: proxies, mode: mode, currentIdx: -1}
}

func (p *proxyRuntime) Start(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.locked {
		return p.current, nil
	}
	if len(p.proxies) == 0 {
		p.locked = true
		p.current = ""
		return "", nil
	}
	indexes := p.indexOrderLocked()
	for _, idx := range indexes {
		candidate := p.proxies[idx]
		result := proxypool.Test(ctx, candidate, 15*time.Second)
		if result.OK {
			p.currentIdx = idx
			p.current = candidate
			return candidate, nil
		}
		if p.mode == "random" {
			message := strings.TrimSpace(result.Error)
			if message == "" {
				message = "代理不可用"
			}
			return "", fmt.Errorf("代理测试失败: %s", message)
		}
	}
	return "", fmt.Errorf("当前分组全部代理测试失败")
}

func (p *proxyRuntime) CurrentProxy() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.current
}

func (p *proxyRuntime) HandleRequestFailure(target string, err error) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.locked || len(p.proxies) == 0 || p.mode != "round_robin" {
		return p.current, false
	}
	nextIdx := p.currentIdx + 1
	if nextIdx < 0 {
		nextIdx = 0
	}
	if nextIdx >= len(p.proxies) {
		p.locked = true
		return p.current, false
	}
	p.currentIdx = nextIdx
	p.current = p.proxies[nextIdx]
	return p.current, true
}

func (p *proxyRuntime) indexOrderLocked() []int {
	if len(p.proxies) == 0 {
		return nil
	}
	if p.mode == "random" {
		idx := rand.Intn(len(p.proxies))
		return []int{idx}
	}
	indexes := make([]int, 0, len(p.proxies))
	for idx := range p.proxies {
		indexes = append(indexes, idx)
	}
	return indexes
}
