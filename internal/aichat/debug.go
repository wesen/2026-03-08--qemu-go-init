package aichat

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	aisettings "github.com/go-go-golems/geppetto/pkg/steps/ai/settings"
	aitypes "github.com/go-go-golems/geppetto/pkg/steps/ai/types"
	"github.com/go-go-golems/pinocchio/pkg/ui/profileswitch"
)

const (
	defaultOpenAIBaseURL = "https://api.openai.com/v1"
	maxBodyPreviewBytes  = 32 * 1024
	maxRawSnapshotBytes  = 32 * 1024
)

type runtimeDetails struct {
	configHome        string
	profileRegistries string
	profileSlug       string
	baseSettings      *aisettings.StepSettings
	profiles          []profileswitch.ProfileListItem
	resolved          profileswitch.Resolved
}

type FileSnapshot struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	Size   int64  `json:"size"`
	Raw    string `json:"raw,omitempty"`
	Error  string `json:"error,omitempty"`
}

type RegistrySourceSnapshot struct {
	Raw   string            `json:"raw"`
	Kind  string            `json:"kind"`
	Path  string            `json:"path,omitempty"`
	DSN   string            `json:"dsn,omitempty"`
	File  FileSnapshot      `json:"file"`
	Stats map[string]string `json:"stats,omitempty"`
}

type ProfileSummary struct {
	RegistrySlug string `json:"registrySlug"`
	ProfileSlug  string `json:"profileSlug"`
	DisplayName  string `json:"displayName,omitempty"`
	Description  string `json:"description,omitempty"`
	IsDefault    bool   `json:"isDefault"`
	Version      uint64 `json:"version"`
}

type ProviderDebug struct {
	APIType            string            `json:"apiType"`
	Engine             string            `json:"engine,omitempty"`
	BaseURL            string            `json:"baseUrl,omitempty"`
	ChatRequestURL     string            `json:"chatRequestUrl,omitempty"`
	HTTPSProbeURL      string            `json:"httpsProbeUrl,omitempty"`
	SelectedAPIKeyName string            `json:"selectedApiKeyName,omitempty"`
	SelectedAPIKey     string            `json:"selectedApiKey,omitempty"`
	AvailableAPIKeys   map[string]string `json:"availableApiKeys,omitempty"`
	AvailableBaseURLs  map[string]string `json:"availableBaseUrls,omitempty"`
	Organization       string            `json:"organization,omitempty"`
	UserAgent          string            `json:"userAgent,omitempty"`
	Timeout            string            `json:"timeout"`
	UsesHTTPS          bool              `json:"usesHttps"`
	Notes              []string          `json:"notes,omitempty"`
}

type RuntimeDebugSnapshot struct {
	GeneratedAt        string                   `json:"generatedAt"`
	ConfigHome         string                   `json:"configHome"`
	ConfigFile         FileSnapshot             `json:"configFile"`
	Persistence        PersistenceDebugSnapshot `json:"persistence"`
	ProfileRegistries  string                   `json:"profileRegistries"`
	ProfileSlug        string                   `json:"profileSlug"`
	RegistrySources    []RegistrySourceSnapshot `json:"registrySources"`
	AvailableProfiles  []ProfileSummary         `json:"availableProfiles"`
	ResolvedRegistry   string                   `json:"resolvedRegistry"`
	ResolvedProfile    string                   `json:"resolvedProfile"`
	RuntimeKey         string                   `json:"runtimeKey,omitempty"`
	RuntimeFingerprint string                   `json:"runtimeFingerprint,omitempty"`
	SystemPrompt       string                   `json:"systemPrompt,omitempty"`
	StepSettingsPatch  map[string]any           `json:"stepSettingsPatch,omitempty"`
	ProfileMetadata    map[string]any           `json:"profileMetadata,omitempty"`
	EffectiveSettings  *aisettings.StepSettings `json:"effectiveSettings,omitempty"`
	Provider           ProviderDebug            `json:"provider"`
}

type PersistenceDebugSnapshot struct {
	Root                      string       `json:"root"`
	ConversationID            string       `json:"conversationId"`
	TurnsDB                   FileSnapshot `json:"turnsDb"`
	TurnsCount                int64        `json:"turnsCount,omitempty"`
	TurnsCountError           string       `json:"turnsCountError,omitempty"`
	TimelineDB                FileSnapshot `json:"timelineDb"`
	TimelineConversationCount int64        `json:"timelineConversationCount,omitempty"`
	TimelineVersionCount      int64        `json:"timelineVersionCount,omitempty"`
	TimelineEntityCount       int64        `json:"timelineEntityCount,omitempty"`
	TimelineCountError        string       `json:"timelineCountError,omitempty"`
}

type HTTPTraceEvent struct {
	At     string `json:"at"`
	Event  string `json:"event"`
	Detail string `json:"detail,omitempty"`
}

type HTTPSProbeResult struct {
	GeneratedAt     string              `json:"generatedAt"`
	Provider        ProviderDebug       `json:"provider"`
	Method          string              `json:"method"`
	RequestURL      string              `json:"requestUrl"`
	Duration        string              `json:"duration"`
	StatusCode      int                 `json:"statusCode,omitempty"`
	Status          string              `json:"status,omitempty"`
	ResponseHeaders map[string][]string `json:"responseHeaders,omitempty"`
	BodyPreview     string              `json:"bodyPreview,omitempty"`
	Trace           []HTTPTraceEvent    `json:"trace,omitempty"`
	Error           string              `json:"error,omitempty"`
}

func DebugSnapshot(ctx context.Context, options Options) (*RuntimeDebugSnapshot, error) {
	details, err := loadRuntimeDetails(ctx, options)
	if err != nil {
		return nil, err
	}

	configFile := snapshotFile(filepath.Join(details.configHome, "pinocchio", "config.yaml"))
	registrySources, err := snapshotRegistrySources(details.profileRegistries)
	if err != nil {
		return nil, err
	}

	profiles := make([]ProfileSummary, 0, len(details.profiles))
	for _, item := range details.profiles {
		profiles = append(profiles, ProfileSummary{
			RegistrySlug: item.RegistrySlug.String(),
			ProfileSlug:  item.ProfileSlug.String(),
			DisplayName:  item.DisplayName,
			Description:  item.Description,
			IsDefault:    item.IsDefault,
			Version:      item.Version,
		})
	}

	persistence := PersistenceDebugSnapshot{
		Root:           defaultChatStateRoot(firstNonEmpty(options.ChatStateRoot, options.StateRoot)),
		ConversationID: firstNonEmpty(options.ConversationID, defaultConversationID),
		TurnsDB:        snapshotFile(filepath.Join(defaultChatStateRoot(firstNonEmpty(options.ChatStateRoot, options.StateRoot)), "turns.db")),
		TimelineDB:     snapshotFile(filepath.Join(defaultChatStateRoot(firstNonEmpty(options.ChatStateRoot, options.StateRoot)), "timeline.db")),
	}
	if counts, err := queryTurnsCount(persistence.TurnsDB.Path); err != nil {
		persistence.TurnsCountError = err.Error()
	} else {
		persistence.TurnsCount = counts
	}
	if counts, err := queryTimelineCounts(persistence.TimelineDB.Path); err != nil {
		persistence.TimelineCountError = err.Error()
	} else {
		persistence.TimelineConversationCount = counts.Conversations
		persistence.TimelineVersionCount = counts.Versions
		persistence.TimelineEntityCount = counts.Entities
	}

	return &RuntimeDebugSnapshot{
		GeneratedAt:        time.Now().UTC().Format(time.RFC3339),
		ConfigHome:         details.configHome,
		ConfigFile:         configFile,
		Persistence:        persistence,
		ProfileRegistries:  details.profileRegistries,
		ProfileSlug:        details.profileSlug,
		RegistrySources:    registrySources,
		AvailableProfiles:  profiles,
		ResolvedRegistry:   details.resolved.RegistrySlug.String(),
		ResolvedProfile:    details.resolved.ProfileSlug.String(),
		RuntimeKey:         details.resolved.RuntimeKey.String(),
		RuntimeFingerprint: details.resolved.RuntimeFingerprint,
		SystemPrompt:       details.resolved.SystemPrompt,
		StepSettingsPatch:  details.resolved.StepSettingsPatch,
		ProfileMetadata:    details.resolved.Metadata,
		EffectiveSettings:  details.resolved.EffectiveStepSettings,
		Provider:           providerDebug(details.resolved.EffectiveStepSettings),
	}, nil
}

type timelineCounts struct {
	Conversations int64
	Versions      int64
	Entities      int64
}

func queryTurnsCount(path string) (int64, error) {
	if strings.TrimSpace(path) == "" {
		return 0, nil
	}
	db, err := openReadOnlySQLite(path)
	if err != nil {
		return 0, err
	}
	if db == nil {
		return 0, nil
	}
	defer func() { _ = db.Close() }()

	var count int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM turns`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func queryTimelineCounts(path string) (timelineCounts, error) {
	if strings.TrimSpace(path) == "" {
		return timelineCounts{}, nil
	}
	db, err := openReadOnlySQLite(path)
	if err != nil {
		return timelineCounts{}, err
	}
	if db == nil {
		return timelineCounts{}, nil
	}
	defer func() { _ = db.Close() }()

	var counts timelineCounts
	if err := db.QueryRow(`SELECT COUNT(*) FROM timeline_conversations`).Scan(&counts.Conversations); err != nil {
		return timelineCounts{}, err
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM timeline_versions`).Scan(&counts.Versions); err != nil {
		return timelineCounts{}, err
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM timeline_entities`).Scan(&counts.Entities); err != nil {
		return timelineCounts{}, err
	}
	return counts, nil
}

func openReadOnlySQLite(path string) (*sql.DB, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	dsn := fmt.Sprintf("file:%s?mode=ro&_busy_timeout=5000", path)
	return sql.Open("sqlite3", dsn)
}

func ProbeProviderHTTPS(ctx context.Context, options Options) (*HTTPSProbeResult, error) {
	details, err := loadRuntimeDetails(ctx, options)
	if err != nil {
		return nil, err
	}

	provider := providerDebug(details.resolved.EffectiveStepSettings)
	timeout := providerTimeout(details.resolved.EffectiveStepSettings)
	client := &http.Client{Timeout: timeout}
	return probeProviderHTTPSWithClient(ctx, provider, client)
}

func loadRuntimeDetails(ctx context.Context, options Options) (*runtimeDetails, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	configHome, profileRegistries, profileSlug, err := resolveRuntime(options)
	if err != nil {
		return nil, err
	}

	base, err := resolveBaseStepSettings(configHome)
	if err != nil {
		return nil, fmt.Errorf("resolve pinocchio base settings: %w", err)
	}

	manager, err := profileswitch.NewManagerFromSources(ctx, profileRegistries, base)
	if err != nil {
		return nil, fmt.Errorf("load pinocchio profiles: %w", err)
	}
	defer func() { _ = manager.Close() }()

	profiles, err := manager.ListProfiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list pinocchio profiles: %w", err)
	}

	resolved, err := manager.Switch(ctx, profileSlug)
	if err != nil {
		return nil, fmt.Errorf("resolve pinocchio profile %q: %w", profileSlug, err)
	}

	return &runtimeDetails{
		configHome:        configHome,
		profileRegistries: profileRegistries,
		profileSlug:       profileSlug,
		baseSettings:      base,
		profiles:          profiles,
		resolved:          resolved,
	}, nil
}

func snapshotRegistrySources(raw string) ([]RegistrySourceSnapshot, error) {
	entries, err := gepprofiles.ParseProfileRegistrySourceEntries(raw)
	if err != nil {
		return nil, err
	}
	specs, err := gepprofiles.ParseRegistrySourceSpecs(entries)
	if err != nil {
		return nil, err
	}

	ret := make([]RegistrySourceSnapshot, 0, len(specs))
	for _, spec := range specs {
		item := RegistrySourceSnapshot{
			Raw:  spec.Raw,
			Kind: string(spec.Kind),
			Path: spec.Path,
			DSN:  spec.DSN,
			File: snapshotFile(spec.Path),
		}
		if spec.Kind == gepprofiles.RegistrySourceKindSQLiteDSN {
			item.File = FileSnapshot{}
			item.Stats = map[string]string{"dsn": spec.DSN}
		}
		ret = append(ret, item)
	}
	return ret, nil
}

func snapshotFile(path string) FileSnapshot {
	path = strings.TrimSpace(path)
	if path == "" {
		return FileSnapshot{}
	}

	info, err := os.Stat(path)
	if err != nil {
		return FileSnapshot{
			Path:  path,
			Error: err.Error(),
		}
	}

	snapshot := FileSnapshot{
		Path:   path,
		Exists: true,
		Size:   info.Size(),
	}
	if info.IsDir() {
		snapshot.Error = "path is a directory"
		return snapshot
	}

	body, err := os.ReadFile(path)
	if err != nil {
		snapshot.Error = err.Error()
		return snapshot
	}
	if shouldExposeRawSnapshot(path, body) {
		snapshot.Raw = string(body)
	}
	return snapshot
}

func shouldExposeRawSnapshot(path string, body []byte) bool {
	if len(body) == 0 || len(body) > maxRawSnapshotBytes {
		return false
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".db", ".wal", ".shm", ".ko", ".img":
		return false
	}
	return !bytes.Contains(body, []byte{0})
}

func providerDebug(settings *aisettings.StepSettings) ProviderDebug {
	provider := ProviderDebug{
		Timeout:           providerTimeout(settings).String(),
		AvailableAPIKeys:  map[string]string{},
		AvailableBaseURLs: map[string]string{},
	}
	if settings == nil {
		provider.Notes = []string{"step settings are nil"}
		return provider
	}
	if settings.API != nil {
		for key, value := range settings.API.APIKeys {
			provider.AvailableAPIKeys[key] = value
		}
		for key, value := range settings.API.BaseUrls {
			provider.AvailableBaseURLs[key] = value
		}
	}
	if settings.Chat != nil {
		if settings.Chat.ApiType != nil {
			provider.APIType = string(*settings.Chat.ApiType)
		}
		if settings.Chat.Engine != nil {
			provider.Engine = *settings.Chat.Engine
		}
	}
	if settings.Client != nil {
		if settings.Client.Organization != nil {
			provider.Organization = strings.TrimSpace(*settings.Client.Organization)
		}
		if settings.Client.UserAgent != nil {
			provider.UserAgent = strings.TrimSpace(*settings.Client.UserAgent)
		}
	}

	notes := []string{}
	switch provider.APIType {
	case string(aitypes.ApiTypeOpenAI):
		provider.BaseURL = lookupBaseURL(settings, "openai-base-url", defaultOpenAIBaseURL)
		provider.ChatRequestURL = joinURL(provider.BaseURL, "/chat/completions")
		provider.HTTPSProbeURL = joinURL(provider.BaseURL, "/models")
		provider.SelectedAPIKeyName = "openai-api-key"
		provider.SelectedAPIKey = lookupAPIKey(settings, provider.SelectedAPIKeyName)
	case string(aitypes.ApiTypeOpenAIResponses):
		provider.BaseURL = lookupBaseURL(settings, "openai-base-url", defaultOpenAIBaseURL)
		provider.ChatRequestURL = joinURL(provider.BaseURL, "/responses")
		provider.HTTPSProbeURL = joinURL(provider.BaseURL, "/models")
		provider.SelectedAPIKeyName = "openai-api-key"
		provider.SelectedAPIKey = lookupAPIKey(settings, "openai-api-key")
		if provider.SelectedAPIKey == "" {
			if fallback := lookupAPIKey(settings, "openai-responses-api-key"); fallback != "" {
				notes = append(notes, "openai-responses-api-key is present, but the current openai_responses engine reads openai-api-key")
			}
		}
	case "":
		notes = append(notes, "chat api type is empty")
	default:
		baseURLKey := provider.APIType + "-base-url"
		apiKeyName := provider.APIType + "-api-key"
		provider.BaseURL = lookupBaseURL(settings, baseURLKey, "")
		provider.ChatRequestURL = joinURL(provider.BaseURL, "/chat/completions")
		provider.HTTPSProbeURL = joinURL(provider.BaseURL, "/models")
		provider.SelectedAPIKeyName = apiKeyName
		provider.SelectedAPIKey = lookupAPIKey(settings, apiKeyName)
	}

	if provider.BaseURL == "" {
		notes = append(notes, "resolved provider base URL is empty")
	}
	if parsed, err := url.Parse(provider.BaseURL); err == nil {
		provider.UsesHTTPS = strings.EqualFold(parsed.Scheme, "https")
	}
	if provider.SelectedAPIKey == "" {
		notes = append(notes, fmt.Sprintf("resolved API key %q is empty", provider.SelectedAPIKeyName))
	}
	provider.Notes = notes
	return provider
}

func lookupAPIKey(settings *aisettings.StepSettings, key string) string {
	if settings == nil || settings.API == nil {
		return ""
	}
	return strings.TrimSpace(settings.API.APIKeys[key])
}

func lookupBaseURL(settings *aisettings.StepSettings, key string, fallback string) string {
	if settings == nil || settings.API == nil {
		return fallback
	}
	if value := strings.TrimSpace(settings.API.BaseUrls[key]); value != "" {
		return value
	}
	return fallback
}

func joinURL(baseURL string, suffix string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		return ""
	}
	return trimmed + suffix
}

func providerTimeout(settings *aisettings.StepSettings) time.Duration {
	if settings != nil && settings.Client != nil && settings.Client.Timeout != nil && *settings.Client.Timeout > 0 {
		return *settings.Client.Timeout
	}
	return 15 * time.Second
}

func probeProviderHTTPSWithClient(ctx context.Context, provider ProviderDebug, client *http.Client) (*HTTPSProbeResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	if strings.TrimSpace(provider.HTTPSProbeURL) == "" {
		return nil, fmt.Errorf("provider probe URL is empty")
	}

	timeout := client.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	started := time.Now()
	traceEvents := []HTTPTraceEvent{}
	appendTrace := func(event string, detail string) {
		traceEvents = append(traceEvents, HTTPTraceEvent{
			At:     time.Since(started).Round(time.Millisecond).String(),
			Event:  event,
			Detail: detail,
		})
	}

	trace := &httptrace.ClientTrace{
		DNSStart: func(info httptrace.DNSStartInfo) {
			appendTrace("dns_start", info.Host)
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			detail := fmt.Sprintf("coalesced=%t addrs=%v err=%v", info.Coalesced, info.Addrs, info.Err)
			appendTrace("dns_done", detail)
		},
		ConnectStart: func(network string, addr string) {
			appendTrace("connect_start", network+" "+addr)
		},
		ConnectDone: func(network string, addr string, err error) {
			appendTrace("connect_done", fmt.Sprintf("%s %s err=%v", network, addr, err))
		},
		TLSHandshakeStart: func() {
			appendTrace("tls_handshake_start", "")
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			appendTrace("tls_handshake_done", fmt.Sprintf("version=%x did_resume=%t err=%v", state.Version, state.DidResume, err))
		},
		GotConn: func(info httptrace.GotConnInfo) {
			appendTrace("got_conn", fmt.Sprintf("reused=%t was_idle=%t", info.Reused, info.WasIdle))
		},
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			appendTrace("wrote_request", fmt.Sprintf("err=%v", info.Err))
		},
		GotFirstResponseByte: func() {
			appendTrace("first_response_byte", "")
		},
	}

	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(runCtx, trace), http.MethodGet, provider.HTTPSProbeURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if provider.SelectedAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+provider.SelectedAPIKey)
	}
	if strings.TrimSpace(provider.Organization) != "" {
		req.Header.Set("OpenAI-Organization", provider.Organization)
	}
	if strings.TrimSpace(provider.UserAgent) != "" {
		req.Header.Set("User-Agent", provider.UserAgent)
	} else {
		req.Header.Set("User-Agent", "qemu-go-init-aichat-debug/1")
	}

	resp, err := client.Do(req)
	if err != nil {
		appendTrace("request_error", err.Error())
		return &HTTPSProbeResult{
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			Provider:    provider,
			Method:      http.MethodGet,
			RequestURL:  provider.HTTPSProbeURL,
			Duration:    time.Since(started).Round(time.Millisecond).String(),
			Trace:       traceEvents,
			Error:       err.Error(),
		}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	preview, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodyPreviewBytes))
	if readErr != nil {
		appendTrace("read_error", readErr.Error())
	}

	return &HTTPSProbeResult{
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
		Provider:        provider,
		Method:          http.MethodGet,
		RequestURL:      provider.HTTPSProbeURL,
		Duration:        time.Since(started).Round(time.Millisecond).String(),
		StatusCode:      resp.StatusCode,
		Status:          resp.Status,
		ResponseHeaders: resp.Header.Clone(),
		BodyPreview:     string(preview),
		Trace:           traceEvents,
		Error: func() string {
			if readErr != nil {
				return readErr.Error()
			}
			return ""
		}(),
	}, nil
}
