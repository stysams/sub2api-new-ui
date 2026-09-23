package securityaudit

import "github.com/google/wire"

// ProvideCoordinator makes the PromptService's recorder role explicit to Wire;
// NewCoordinator keeps a variadic parameter for callers that do not record.
func ProvideCoordinator(legacy LegacyEngine, prompt *PromptService) *Coordinator {
	return NewCoordinator(legacy, prompt, prompt)
}

var ProviderSet = wire.NewSet(
	NewPostgreSQLRepository,
	wire.Bind(new(JobRepository), new(*PostgreSQLRepository)),
	wire.Bind(new(EventRepository), new(*PostgreSQLRepository)),
	NewRedisPayloadStore,
	wire.Bind(new(PayloadStore), new(*RedisPayloadStore)),
	NewOpenAICompatibleScanner,
	wire.Bind(new(PromptScanner), new(*OpenAICompatibleScanner)),
	NewAtomicMetrics,
	wire.Bind(new(Metrics), new(*AtomicMetrics)),
	NewConfigManager,
	wire.Bind(new(ConfigStore), new(*ConfigManager)),
	NewPromptService,
	wire.Bind(new(PromptEngine), new(*PromptService)),
	wire.Bind(new(PromptAdminService), new(*PromptService)),
	NewLegacyModerationAdapter,
	ProvideCoordinator,
	NewPromptAdminHandler,
)
