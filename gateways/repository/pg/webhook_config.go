package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mailvault/mailvault/domain/entities"
	"github.com/mailvault/mailvault/domain/webhook_config"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type WebhookConfigRepository struct {
	db DBTX
}

func NewWebhookConfigRepository(db DBTX) webhook_config.Repository {
	return &WebhookConfigRepository{
		db: db,
	}
}

// Create creates a new webhook configuration
func (r *WebhookConfigRepository) Create(ctx context.Context, config *entities.WebhookConfiguration) error {
	customHeadersJSON, err := json.Marshal(config.CustomHeaders)
	if err != nil {
		return fmt.Errorf("marshal custom headers: %w", err)
	}

	query := `
		INSERT INTO webhook_configurations (
			id, domain_id, name, description, url, method, enabled, verified,
			auth_type, auth_secret, auth_username, custom_headers, event_types,
			timeout_seconds, max_retries, retry_backoff_multiplier, initial_retry_delay_seconds,
			rate_limit_per_minute, rate_limit_per_hour,
			circuit_breaker_enabled, circuit_breaker_threshold, circuit_breaker_timeout_seconds,
			circuit_breaker_state, circuit_breaker_opened_at,
			health_status, last_health_check_at, last_success_at, last_failure_at,
			consecutive_failures, total_success_count, total_failure_count, average_response_time_ms,
			last_used_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13,
			$14, $15, $16, $17,
			$18, $19,
			$20, $21, $22, $23, $24,
			$25, $26, $27, $28,
			$29, $30, $31, $32,
			$33, $34, $35
		)
	`

	_, err = r.db.Exec(ctx, query,
		config.ID, config.DomainID, config.Name, config.Description, config.URL, config.Method, config.Enabled, config.Verified,
		config.AuthType, config.AuthSecret, config.AuthUsername, customHeadersJSON, config.EventTypes,
		config.TimeoutSeconds, config.MaxRetries, config.RetryBackoffMultiplier, config.InitialRetryDelaySeconds,
		config.RateLimitPerMinute, config.RateLimitPerHour,
		config.CircuitBreakerEnabled, config.CircuitBreakerThreshold, config.CircuitBreakerTimeoutSeconds,
		config.CircuitBreakerState, config.CircuitBreakerOpenedAt,
		config.HealthStatus, config.LastHealthCheckAt, config.LastSuccessAt, config.LastFailureAt,
		config.ConsecutiveFailures, config.TotalSuccessCount, config.TotalFailureCount, config.AverageResponseTimeMs,
		config.LastUsedAt, config.CreatedAt, config.UpdatedAt,
	)

	return err
}

// GetByID retrieves a webhook configuration by ID
func (r *WebhookConfigRepository) GetByID(ctx context.Context, id uuid.UUID) (*entities.WebhookConfiguration, error) {
	query := `
		SELECT
			id, domain_id, name, description, url, method, enabled, verified,
			auth_type, auth_secret, auth_username, custom_headers, event_types,
			timeout_seconds, max_retries, retry_backoff_multiplier, initial_retry_delay_seconds,
			rate_limit_per_minute, rate_limit_per_hour,
			circuit_breaker_enabled, circuit_breaker_threshold, circuit_breaker_timeout_seconds,
			circuit_breaker_state, circuit_breaker_opened_at,
			health_status, last_health_check_at, last_success_at, last_failure_at,
			consecutive_failures, total_success_count, total_failure_count, average_response_time_ms,
			last_used_at, created_at, updated_at
		FROM webhook_configurations
		WHERE id = $1
	`

	return r.scanWebhookConfig(r.db.QueryRow(ctx, query, id))
}

// GetByDomainID retrieves all webhook configurations for a domain
func (r *WebhookConfigRepository) GetByDomainID(ctx context.Context, domainID uuid.UUID) ([]*entities.WebhookConfiguration, error) {
	query := `
		SELECT
			id, domain_id, name, description, url, method, enabled, verified,
			auth_type, auth_secret, auth_username, custom_headers, event_types,
			timeout_seconds, max_retries, retry_backoff_multiplier, initial_retry_delay_seconds,
			rate_limit_per_minute, rate_limit_per_hour,
			circuit_breaker_enabled, circuit_breaker_threshold, circuit_breaker_timeout_seconds,
			circuit_breaker_state, circuit_breaker_opened_at,
			health_status, last_health_check_at, last_success_at, last_failure_at,
			consecutive_failures, total_success_count, total_failure_count, average_response_time_ms,
			last_used_at, created_at, updated_at
		FROM webhook_configurations
		WHERE domain_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*entities.WebhookConfiguration
	for rows.Next() {
		config, err := r.scanWebhookConfigFromRows(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}

	return configs, rows.Err()
}

// GetActiveByDomainID retrieves all active webhook configurations for a domain
func (r *WebhookConfigRepository) GetActiveByDomainID(ctx context.Context, domainID uuid.UUID) ([]*entities.WebhookConfiguration, error) {
	query := `
		SELECT
			id, domain_id, name, description, url, method, enabled, verified,
			auth_type, auth_secret, auth_username, custom_headers, event_types,
			timeout_seconds, max_retries, retry_backoff_multiplier, initial_retry_delay_seconds,
			rate_limit_per_minute, rate_limit_per_hour,
			circuit_breaker_enabled, circuit_breaker_threshold, circuit_breaker_timeout_seconds,
			circuit_breaker_state, circuit_breaker_opened_at,
			health_status, last_health_check_at, last_success_at, last_failure_at,
			consecutive_failures, total_success_count, total_failure_count, average_response_time_ms,
			last_used_at, created_at, updated_at
		FROM webhook_configurations
		WHERE domain_id = $1 AND enabled = true AND circuit_breaker_state != 'open'
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, domainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*entities.WebhookConfiguration
	for rows.Next() {
		config, err := r.scanWebhookConfigFromRows(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}

	return configs, rows.Err()
}

// Update updates a webhook configuration
func (r *WebhookConfigRepository) Update(ctx context.Context, config *entities.WebhookConfiguration) error {
	customHeadersJSON, err := json.Marshal(config.CustomHeaders)
	if err != nil {
		return fmt.Errorf("marshal custom headers: %w", err)
	}

	query := `
		UPDATE webhook_configurations SET
			name = $2, description = $3, url = $4, method = $5, enabled = $6, verified = $7,
			auth_type = $8, auth_secret = $9, auth_username = $10, custom_headers = $11, event_types = $12,
			timeout_seconds = $13, max_retries = $14, retry_backoff_multiplier = $15, initial_retry_delay_seconds = $16,
			rate_limit_per_minute = $17, rate_limit_per_hour = $18,
			circuit_breaker_enabled = $19, circuit_breaker_threshold = $20, circuit_breaker_timeout_seconds = $21,
			circuit_breaker_state = $22, circuit_breaker_opened_at = $23,
			health_status = $24, last_health_check_at = $25, last_success_at = $26, last_failure_at = $27,
			consecutive_failures = $28, total_success_count = $29, total_failure_count = $30, average_response_time_ms = $31,
			last_used_at = $32, updated_at = $33
		WHERE id = $1
	`

	result, err := r.db.Exec(ctx, query,
		config.ID, config.Name, config.Description, config.URL, config.Method, config.Enabled, config.Verified,
		config.AuthType, config.AuthSecret, config.AuthUsername, customHeadersJSON, config.EventTypes,
		config.TimeoutSeconds, config.MaxRetries, config.RetryBackoffMultiplier, config.InitialRetryDelaySeconds,
		config.RateLimitPerMinute, config.RateLimitPerHour,
		config.CircuitBreakerEnabled, config.CircuitBreakerThreshold, config.CircuitBreakerTimeoutSeconds,
		config.CircuitBreakerState, config.CircuitBreakerOpenedAt,
		config.HealthStatus, config.LastHealthCheckAt, config.LastSuccessAt, config.LastFailureAt,
		config.ConsecutiveFailures, config.TotalSuccessCount, config.TotalFailureCount, config.AverageResponseTimeMs,
		config.LastUsedAt, config.UpdatedAt,
	)

	if err != nil {
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// Delete deletes a webhook configuration
func (r *WebhookConfigRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM webhook_configurations WHERE id = $1`
	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// GetByDomainIDAndName retrieves a webhook configuration by domain ID and name
func (r *WebhookConfigRepository) GetByDomainIDAndName(ctx context.Context, domainID uuid.UUID, name string) (*entities.WebhookConfiguration, error) {
	query := `
		SELECT
			id, domain_id, name, description, url, method, enabled, verified,
			auth_type, auth_secret, auth_username, custom_headers, event_types,
			timeout_seconds, max_retries, retry_backoff_multiplier, initial_retry_delay_seconds,
			rate_limit_per_minute, rate_limit_per_hour,
			circuit_breaker_enabled, circuit_breaker_threshold, circuit_breaker_timeout_seconds,
			circuit_breaker_state, circuit_breaker_opened_at,
			health_status, last_health_check_at, last_success_at, last_failure_at,
			consecutive_failures, total_success_count, total_failure_count, average_response_time_ms,
			last_used_at, created_at, updated_at
		FROM webhook_configurations
		WHERE domain_id = $1 AND name = $2
	`

	return r.scanWebhookConfig(r.db.QueryRow(ctx, query, domainID, name))
}

// ListUnhealthyWebhooks retrieves webhooks with unhealthy status
func (r *WebhookConfigRepository) ListUnhealthyWebhooks(ctx context.Context, limit int) ([]*entities.WebhookConfiguration, error) {
	query := `
		SELECT
			id, domain_id, name, description, url, method, enabled, verified,
			auth_type, auth_secret, auth_username, custom_headers, event_types,
			timeout_seconds, max_retries, retry_backoff_multiplier, initial_retry_delay_seconds,
			rate_limit_per_minute, rate_limit_per_hour,
			circuit_breaker_enabled, circuit_breaker_threshold, circuit_breaker_timeout_seconds,
			circuit_breaker_state, circuit_breaker_opened_at,
			health_status, last_health_check_at, last_success_at, last_failure_at,
			consecutive_failures, total_success_count, total_failure_count, average_response_time_ms,
			last_used_at, created_at, updated_at
		FROM webhook_configurations
		WHERE health_status IN ('unhealthy', 'degraded')
		ORDER BY last_health_check_at ASC NULLS FIRST
		LIMIT $1
	`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*entities.WebhookConfiguration
	for rows.Next() {
		config, err := r.scanWebhookConfigFromRows(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}

	return configs, rows.Err()
}

// ListWebhooksForHealthCheck retrieves webhooks that need health checks
func (r *WebhookConfigRepository) ListWebhooksForHealthCheck(ctx context.Context, olderThanMinutes int) ([]*entities.WebhookConfiguration, error) {
	query := `
		SELECT
			id, domain_id, name, description, url, method, enabled, verified,
			auth_type, auth_secret, auth_username, custom_headers, event_types,
			timeout_seconds, max_retries, retry_backoff_multiplier, initial_retry_delay_seconds,
			rate_limit_per_minute, rate_limit_per_hour,
			circuit_breaker_enabled, circuit_breaker_threshold, circuit_breaker_timeout_seconds,
			circuit_breaker_state, circuit_breaker_opened_at,
			health_status, last_health_check_at, last_success_at, last_failure_at,
			consecutive_failures, total_success_count, total_failure_count, average_response_time_ms,
			last_used_at, created_at, updated_at
		FROM webhook_configurations
		WHERE enabled = true
		  AND (last_health_check_at IS NULL
		       OR last_health_check_at < NOW() - INTERVAL '1 minute' * $1)
		ORDER BY last_health_check_at ASC NULLS FIRST
		LIMIT 100
	`

	rows, err := r.db.Query(ctx, query, olderThanMinutes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*entities.WebhookConfiguration
	for rows.Next() {
		config, err := r.scanWebhookConfigFromRows(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}

	return configs, rows.Err()
}

// CreateAudit creates an audit log entry
func (r *WebhookConfigRepository) CreateAudit(ctx context.Context, audit *entities.WebhookConfigurationAudit) error {
	oldValuesJSON, err := json.Marshal(audit.OldValues)
	if err != nil {
		return fmt.Errorf("marshal old values: %w", err)
	}

	newValuesJSON, err := json.Marshal(audit.NewValues)
	if err != nil {
		return fmt.Errorf("marshal new values: %w", err)
	}

	query := `
		INSERT INTO webhook_configuration_audit (
			id, webhook_config_id, changed_by_user_id, action, old_values, new_values,
			change_reason, ip_address, user_agent, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err = r.db.Exec(ctx, query,
		audit.ID, audit.WebhookConfigID, audit.ChangedByUserID, audit.Action,
		oldValuesJSON, newValuesJSON, audit.ChangeReason,
		audit.IPAddress, audit.UserAgent, audit.CreatedAt,
	)

	return err
}

// GetAuditByConfigID retrieves audit logs for a webhook configuration
func (r *WebhookConfigRepository) GetAuditByConfigID(ctx context.Context, configID uuid.UUID, limit, offset int) ([]*entities.WebhookConfigurationAudit, error) {
	query := `
		SELECT
			id, webhook_config_id, changed_by_user_id, action, old_values, new_values,
			change_reason, ip_address, user_agent, created_at
		FROM webhook_configuration_audit
		WHERE webhook_config_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(ctx, query, configID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var audits []*entities.WebhookConfigurationAudit
	for rows.Next() {
		audit, err := r.scanAuditFromRows(rows)
		if err != nil {
			return nil, err
		}
		audits = append(audits, audit)
	}

	return audits, rows.Err()
}

// CreateHealthCheck creates a health check record
func (r *WebhookConfigRepository) CreateHealthCheck(ctx context.Context, check *entities.WebhookHealthCheck) error {
	query := `
		INSERT INTO webhook_health_checks (
			id, webhook_config_id, check_type, status, response_time_ms,
			response_status_code, error_message, checked_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Exec(ctx, query,
		check.ID, check.WebhookConfigID, check.CheckType, check.Status,
		check.ResponseTimeMs, check.ResponseStatusCode, check.ErrorMessage, check.CheckedAt,
	)

	return err
}

// GetHealthChecksByConfigID retrieves health checks for a webhook configuration
func (r *WebhookConfigRepository) GetHealthChecksByConfigID(ctx context.Context, configID uuid.UUID, limit, offset int) ([]*entities.WebhookHealthCheck, error) {
	query := `
		SELECT
			id, webhook_config_id, check_type, status, response_time_ms,
			response_status_code, error_message, checked_at
		FROM webhook_health_checks
		WHERE webhook_config_id = $1
		ORDER BY checked_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(ctx, query, configID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []*entities.WebhookHealthCheck
	for rows.Next() {
		check, err := r.scanHealthCheckFromRows(rows)
		if err != nil {
			return nil, err
		}
		checks = append(checks, check)
	}

	return checks, rows.Err()
}

// GetLatestHealthCheck retrieves the most recent health check
func (r *WebhookConfigRepository) GetLatestHealthCheck(ctx context.Context, configID uuid.UUID) (*entities.WebhookHealthCheck, error) {
	query := `
		SELECT
			id, webhook_config_id, check_type, status, response_time_ms,
			response_status_code, error_message, checked_at
		FROM webhook_health_checks
		WHERE webhook_config_id = $1
		ORDER BY checked_at DESC
		LIMIT 1
	`

	return r.scanHealthCheck(r.db.QueryRow(ctx, query, configID))
}

// GetTemplates retrieves all active webhook templates
func (r *WebhookConfigRepository) GetTemplates(ctx context.Context) ([]*entities.WebhookConfigurationTemplate, error) {
	query := `
		SELECT
			id, name, description, provider_name, default_method, default_auth_type,
			default_headers, default_timeout_seconds, documentation_url, is_active,
			usage_count, created_at, updated_at
		FROM webhook_configuration_templates
		WHERE is_active = true
		ORDER BY usage_count DESC, name ASC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []*entities.WebhookConfigurationTemplate
	for rows.Next() {
		template, err := r.scanTemplateFromRows(rows)
		if err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}

	return templates, rows.Err()
}

// GetTemplateByID retrieves a template by ID
func (r *WebhookConfigRepository) GetTemplateByID(ctx context.Context, id uuid.UUID) (*entities.WebhookConfigurationTemplate, error) {
	query := `
		SELECT
			id, name, description, provider_name, default_method, default_auth_type,
			default_headers, default_timeout_seconds, documentation_url, is_active,
			usage_count, created_at, updated_at
		FROM webhook_configuration_templates
		WHERE id = $1
	`

	return r.scanTemplate(r.db.QueryRow(ctx, query, id))
}

// GetTemplateByName retrieves a template by name
func (r *WebhookConfigRepository) GetTemplateByName(ctx context.Context, name string) (*entities.WebhookConfigurationTemplate, error) {
	query := `
		SELECT
			id, name, description, provider_name, default_method, default_auth_type,
			default_headers, default_timeout_seconds, documentation_url, is_active,
			usage_count, created_at, updated_at
		FROM webhook_configuration_templates
		WHERE name = $1
	`

	return r.scanTemplate(r.db.QueryRow(ctx, query, name))
}

// CreateTemplate creates a new webhook template
func (r *WebhookConfigRepository) CreateTemplate(ctx context.Context, template *entities.WebhookConfigurationTemplate) error {
	defaultHeadersJSON, err := json.Marshal(template.DefaultHeaders)
	if err != nil {
		return fmt.Errorf("marshal default headers: %w", err)
	}

	query := `
		INSERT INTO webhook_configuration_templates (
			id, name, description, provider_name, default_method, default_auth_type,
			default_headers, default_timeout_seconds, documentation_url, is_active,
			usage_count, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	_, err = r.db.Exec(ctx, query,
		template.ID, template.Name, template.Description, template.ProviderName,
		template.DefaultMethod, template.DefaultAuthType, defaultHeadersJSON,
		template.DefaultTimeoutSeconds, template.DocumentationURL, template.IsActive,
		template.UsageCount, template.CreatedAt, template.UpdatedAt,
	)

	return err
}

// UpdateTemplate updates a webhook template
func (r *WebhookConfigRepository) UpdateTemplate(ctx context.Context, template *entities.WebhookConfigurationTemplate) error {
	defaultHeadersJSON, err := json.Marshal(template.DefaultHeaders)
	if err != nil {
		return fmt.Errorf("marshal default headers: %w", err)
	}

	query := `
		UPDATE webhook_configuration_templates SET
			name = $2, description = $3, provider_name = $4, default_method = $5,
			default_auth_type = $6, default_headers = $7, default_timeout_seconds = $8,
			documentation_url = $9, is_active = $10, usage_count = $11, updated_at = $12
		WHERE id = $1
	`

	result, err := r.db.Exec(ctx, query,
		template.ID, template.Name, template.Description, template.ProviderName,
		template.DefaultMethod, template.DefaultAuthType, defaultHeadersJSON,
		template.DefaultTimeoutSeconds, template.DocumentationURL, template.IsActive,
		template.UsageCount, template.UpdatedAt,
	)

	if err != nil {
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// IncrementTemplateUsage increments the usage count for a template
func (r *WebhookConfigRepository) IncrementTemplateUsage(ctx context.Context, templateID uuid.UUID) error {
	query := `
		UPDATE webhook_configuration_templates
		SET usage_count = usage_count + 1, updated_at = NOW()
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query, templateID)
	return err
}

// Helper functions for scanning

func (r *WebhookConfigRepository) scanWebhookConfig(row pgx.Row) (*entities.WebhookConfiguration, error) {
	var config entities.WebhookConfiguration
	var customHeadersJSON []byte
	var eventTypes pgtype.Array[string]

	err := row.Scan(
		&config.ID, &config.DomainID, &config.Name, &config.Description, &config.URL, &config.Method, &config.Enabled, &config.Verified,
		&config.AuthType, &config.AuthSecret, &config.AuthUsername, &customHeadersJSON, &eventTypes,
		&config.TimeoutSeconds, &config.MaxRetries, &config.RetryBackoffMultiplier, &config.InitialRetryDelaySeconds,
		&config.RateLimitPerMinute, &config.RateLimitPerHour,
		&config.CircuitBreakerEnabled, &config.CircuitBreakerThreshold, &config.CircuitBreakerTimeoutSeconds,
		&config.CircuitBreakerState, &config.CircuitBreakerOpenedAt,
		&config.HealthStatus, &config.LastHealthCheckAt, &config.LastSuccessAt, &config.LastFailureAt,
		&config.ConsecutiveFailures, &config.TotalSuccessCount, &config.TotalFailureCount, &config.AverageResponseTimeMs,
		&config.LastUsedAt, &config.CreatedAt, &config.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("scan webhook config: %w", err)
	}

	if len(customHeadersJSON) > 0 {
		if err := json.Unmarshal(customHeadersJSON, &config.CustomHeaders); err != nil {
			return nil, fmt.Errorf("unmarshal custom headers: %w", err)
		}
	}

	config.EventTypes = eventTypes.Elements

	return &config, nil
}

func (r *WebhookConfigRepository) scanWebhookConfigFromRows(rows pgx.Rows) (*entities.WebhookConfiguration, error) {
	var config entities.WebhookConfiguration
	var customHeadersJSON []byte
	var eventTypes pgtype.Array[string]

	err := rows.Scan(
		&config.ID, &config.DomainID, &config.Name, &config.Description, &config.URL, &config.Method, &config.Enabled, &config.Verified,
		&config.AuthType, &config.AuthSecret, &config.AuthUsername, &customHeadersJSON, &eventTypes,
		&config.TimeoutSeconds, &config.MaxRetries, &config.RetryBackoffMultiplier, &config.InitialRetryDelaySeconds,
		&config.RateLimitPerMinute, &config.RateLimitPerHour,
		&config.CircuitBreakerEnabled, &config.CircuitBreakerThreshold, &config.CircuitBreakerTimeoutSeconds,
		&config.CircuitBreakerState, &config.CircuitBreakerOpenedAt,
		&config.HealthStatus, &config.LastHealthCheckAt, &config.LastSuccessAt, &config.LastFailureAt,
		&config.ConsecutiveFailures, &config.TotalSuccessCount, &config.TotalFailureCount, &config.AverageResponseTimeMs,
		&config.LastUsedAt, &config.CreatedAt, &config.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("scan webhook config: %w", err)
	}

	if len(customHeadersJSON) > 0 {
		if err := json.Unmarshal(customHeadersJSON, &config.CustomHeaders); err != nil {
			return nil, fmt.Errorf("unmarshal custom headers: %w", err)
		}
	}

	config.EventTypes = eventTypes.Elements

	return &config, nil
}

func (r *WebhookConfigRepository) scanAuditFromRows(rows pgx.Rows) (*entities.WebhookConfigurationAudit, error) {
	var audit entities.WebhookConfigurationAudit
	var oldValuesJSON, newValuesJSON []byte

	err := rows.Scan(
		&audit.ID, &audit.WebhookConfigID, &audit.ChangedByUserID, &audit.Action,
		&oldValuesJSON, &newValuesJSON, &audit.ChangeReason,
		&audit.IPAddress, &audit.UserAgent, &audit.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("scan audit: %w", err)
	}

	if len(oldValuesJSON) > 0 {
		if err := json.Unmarshal(oldValuesJSON, &audit.OldValues); err != nil {
			return nil, fmt.Errorf("unmarshal old values: %w", err)
		}
	}

	if len(newValuesJSON) > 0 {
		if err := json.Unmarshal(newValuesJSON, &audit.NewValues); err != nil {
			return nil, fmt.Errorf("unmarshal new values: %w", err)
		}
	}

	return &audit, nil
}

func (r *WebhookConfigRepository) scanHealthCheck(row pgx.Row) (*entities.WebhookHealthCheck, error) {
	var check entities.WebhookHealthCheck

	err := row.Scan(
		&check.ID, &check.WebhookConfigID, &check.CheckType, &check.Status,
		&check.ResponseTimeMs, &check.ResponseStatusCode, &check.ErrorMessage, &check.CheckedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("scan health check: %w", err)
	}

	return &check, nil
}

func (r *WebhookConfigRepository) scanHealthCheckFromRows(rows pgx.Rows) (*entities.WebhookHealthCheck, error) {
	var check entities.WebhookHealthCheck

	err := rows.Scan(
		&check.ID, &check.WebhookConfigID, &check.CheckType, &check.Status,
		&check.ResponseTimeMs, &check.ResponseStatusCode, &check.ErrorMessage, &check.CheckedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("scan health check: %w", err)
	}

	return &check, nil
}

func (r *WebhookConfigRepository) scanTemplate(row pgx.Row) (*entities.WebhookConfigurationTemplate, error) {
	var template entities.WebhookConfigurationTemplate
	var defaultHeadersJSON []byte

	err := row.Scan(
		&template.ID, &template.Name, &template.Description, &template.ProviderName,
		&template.DefaultMethod, &template.DefaultAuthType, &defaultHeadersJSON,
		&template.DefaultTimeoutSeconds, &template.DocumentationURL, &template.IsActive,
		&template.UsageCount, &template.CreatedAt, &template.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("scan template: %w", err)
	}

	if len(defaultHeadersJSON) > 0 {
		if err := json.Unmarshal(defaultHeadersJSON, &template.DefaultHeaders); err != nil {
			return nil, fmt.Errorf("unmarshal default headers: %w", err)
		}
	}

	return &template, nil
}

func (r *WebhookConfigRepository) scanTemplateFromRows(rows pgx.Rows) (*entities.WebhookConfigurationTemplate, error) {
	var template entities.WebhookConfigurationTemplate
	var defaultHeadersJSON []byte

	err := rows.Scan(
		&template.ID, &template.Name, &template.Description, &template.ProviderName,
		&template.DefaultMethod, &template.DefaultAuthType, &defaultHeadersJSON,
		&template.DefaultTimeoutSeconds, &template.DocumentationURL, &template.IsActive,
		&template.UsageCount, &template.CreatedAt, &template.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("scan template: %w", err)
	}

	if len(defaultHeadersJSON) > 0 {
		if err := json.Unmarshal(defaultHeadersJSON, &template.DefaultHeaders); err != nil {
			return nil, fmt.Errorf("unmarshal default headers: %w", err)
		}
	}

	return &template, nil
}
