package usecase

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rahmatrdn/go-ch-manager/entity"
	"github.com/rahmatrdn/go-ch-manager/internal/repository/clickhouse"
	"github.com/rahmatrdn/go-ch-manager/internal/repository/sqlite"
)

type ConnectionUsecase struct {
	repo        sqlite.ConnectionRepository
	historyRepo sqlite.QueryHistoryRepository
	favRepo     sqlite.FavoriteRepository
	chClient    clickhouse.ClickHouseClient
}

func NewConnectionUsecase(repo sqlite.ConnectionRepository, historyRepo sqlite.QueryHistoryRepository, favRepo sqlite.FavoriteRepository, chClient clickhouse.ClickHouseClient) *ConnectionUsecase {
	return &ConnectionUsecase{
		repo:        repo,
		historyRepo: historyRepo,
		favRepo:     favRepo,
		chClient:    chClient,
	}
}

func (u *ConnectionUsecase) CreateConnection(ctx context.Context, conn *entity.CHConnection) error {
	conn.CreatedAt = time.Now()
	conn.UpdatedAt = time.Now()
	// Optionally test connection before saving?
	// Try to ping the connection
	if err := u.chClient.Ping(ctx, conn); err != nil {
		return err
	}

	// Fetch and save server info
	info, err := u.chClient.GetServerInfo(ctx, conn)
	if err == nil {
		conn.ServerInfo = info
	}

	return u.repo.Create(ctx, conn)
}

func (u *ConnectionUsecase) UpdateConnection(ctx context.Context, id int64, conn *entity.CHConnection) error {
	existing, err := u.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		// return specific not found error or generic
		return nil // Should probably return error, but keeping simple for now
	}

	conn.ID = id
	conn.UpdatedAt = time.Now()
	conn.CreatedAt = existing.CreatedAt // Preserve created_at

	// Optional: validate connection
	if err := u.chClient.Ping(ctx, conn); err != nil {
		return err
	}

	// Fetch and save server info
	info, err := u.chClient.GetServerInfo(ctx, conn)
	if err == nil {
		conn.ServerInfo = info
	}

	return u.repo.Update(ctx, conn)
}

func (u *ConnectionUsecase) GetAllConnections(ctx context.Context) ([]*entity.CHConnection, error) {
	return u.repo.FindAll(ctx)
}

func (u *ConnectionUsecase) GetConnectionStatus(ctx context.Context, id int64) (string, error) {
	conn, err := u.repo.FindByID(ctx, id)
	if err != nil {
		return "Error", err
	}
	if conn == nil {
		return "Not Found", nil
	}

	// err = u.chClient.Ping(ctx, conn)

	// if err != nil {
	// 	return "Offline", nil
	// }
	return "Online", nil
}

func (u *ConnectionUsecase) GetServerInfo(ctx context.Context, id int64) (string, error) {
	conn, err := u.repo.FindByID(ctx, id)
	if err != nil {
		return "", err
	}
	if conn == nil {
		return "", nil
	}
	return conn.ServerInfo, nil
}

func (u *ConnectionUsecase) GetDatabases(ctx context.Context, id int64) ([]string, error) {
	conn, err := u.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, nil
	}
	return u.chClient.GetDatabases(ctx, conn)
}

func (u *ConnectionUsecase) GetTables(ctx context.Context, id int64, db ...string) ([]entity.TableMeta, error) {
	conn, err := u.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, nil // Or error not found
	}

	// Override database if provided
	if len(db) > 0 && db[0] != "" {
		conn.Database = db[0]
	}

	return u.chClient.GetTables(ctx, conn)
}

func (u *ConnectionUsecase) GetSchema(ctx context.Context, id int64, table string, db ...string) (*entity.TableSchema, string, error) {
	conn, err := u.repo.FindByID(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if conn == nil {
		return nil, "", nil
	}

	// Override database if provided
	if len(db) > 0 && db[0] != "" {
		conn.Database = db[0]
	}

	schema, err := u.chClient.GetSchema(ctx, conn, table)
	if err != nil {
		return nil, "", err
	}

	createSQL, err := u.chClient.GetCreateSQL(ctx, conn, table)
	if err != nil {
		// Non-critical if create SQL fails?
		createSQL = "-- Failed to fetch create SQL"
	}

	return schema, createSQL, nil
}

func (u *ConnectionUsecase) GetQueryHistory(ctx context.Context, connectionID int64) ([]*entity.QueryHistory, error) {
	return u.historyRepo.FindByConnectionID(ctx, connectionID, 50)
}

func (u *ConnectionUsecase) CompareQueries(ctx context.Context, id int64, query1, query2 string) (*entity.CompareResult, error) {
	conn, err := u.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, nil // Or error not found
	}

	stats1, err := u.chClient.ExecuteQueryWithStats(ctx, conn, query1)
	if err != nil {
		return nil, err
	}

	stats2, err := u.chClient.ExecuteQueryWithStats(ctx, conn, query2)
	if err != nil {
		return nil, err
	}

	return &entity.CompareResult{
		Query1Stats: stats1,
		Query2Stats: stats2,
	}, nil
}

func (u *ConnectionUsecase) ExecuteQuery(ctx context.Context, id int64, query string) (*entity.QueryResult, error) {
	conn, err := u.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, nil // Or return not found error
	}

	result, err := u.chClient.ExecuteQueryWithResults(ctx, conn, query)
	if err != nil {
		return nil, err
	}

	// Save to history (Async or Sync? Sync for now to simple)
	go func() {
		// Create a new context for the background task to avoid cancellation if the request context is cancelled
		bgCtx := context.Background()
		history := &entity.QueryHistory{
			ConnectionID: id,
			Query:        query,
		}
		_ = u.historyRepo.Create(bgCtx, history)
		_ = u.historyRepo.Prune(bgCtx, id, 50)
	}()

	return result, nil
}

func (u *ConnectionUsecase) GetConfigurationData(ctx context.Context, id int64) (*entity.ConfigurationData, error) {
	conn, err := u.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, nil
	}

	data := &entity.ConfigurationData{}

	// Fetch data sequentially (could be parallelized)
	info, err := u.chClient.GetClusterConfig(ctx, conn)
	if info != nil {
		data.ClusterInfo = *info
	}
	// We ignore err for ClusterInfo because we want to show partial data (Host/Port) even if connection fails

	if settings, err := u.chClient.GetSettings(ctx, conn); err == nil {
		data.Settings = settings
	}

	if users, err := u.chClient.GetUsers(ctx, conn); err == nil {
		data.Users = users
	}

	if roles, err := u.chClient.GetRoles(ctx, conn); err == nil {
		data.Roles = roles
	}

	if policies, disks, err := u.chClient.GetStoragePolicies(ctx, conn); err == nil {
		data.StoragePolicies = policies
		data.Disks = disks
	}

	if stats, err := u.chClient.GetProcessStats(ctx, conn); err == nil {
		data.Processes = *stats
	}

	if logCfg, err := u.chClient.GetLogConfig(ctx, conn); err == nil {
		data.LogConfig = *logCfg
	}

	return data, nil
}

func (u *ConnectionUsecase) SaveFavoriteComparison(ctx context.Context, fav *entity.FavoriteComparison) error {
	return u.favRepo.Create(ctx, fav)
}

func (u *ConnectionUsecase) GetFavoriteComparisons(ctx context.Context, connectionID int64) ([]*entity.FavoriteComparison, error) {
	return u.favRepo.FindAllByConnectionID(ctx, connectionID)
}

func (u *ConnectionUsecase) DeleteFavoriteComparison(ctx context.Context, id int64) error {
	return u.favRepo.Delete(ctx, id)
}

func (u *ConnectionUsecase) AnalyzeQuery(ctx context.Context, id int64, query string) (*entity.QueryAnalysis, error) {
	conn, err := u.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, fmt.Errorf("connection not found")
	}

	// Set default database if not set
	if conn.Database == "" {
		conn.Database = "default"
	}

	// Run EXPLAIN on the query
	explainPlan, err := u.chClient.ExplainQuery(ctx, conn, query)
	if err != nil {
		explainPlan = fmt.Sprintf("-- Failed to get EXPLAIN plan: %v", err)
	}

	// Execute the query to get stats
	queryStats, err := u.chClient.ExecuteQueryWithStats(ctx, conn, query)
	if err != nil {
		queryStats = nil
	}

	// Extract table references from query (with database)
	tableRefs := u.extractTableReferences(query)

	// Fetch CREATE TABLE statements for each table
	tableSchemas := make([]entity.TableSchemaInfo, 0)
	warnings := make([]string, 0)

	for _, ref := range tableRefs {
		database := ref.Database
		tableName := ref.TableName
		hasWarning := false

		// If database not specified in query, use connection's default database
		if database == "" {
			database = conn.Database
			hasWarning = true
			warnings = append(warnings, fmt.Sprintf("Table '%s' does not have database specified. Using default database '%s'. Please use database_name'.%s' in your query for clarity.", tableName, database, tableName))
		}

		// Create a temporary connection with the database
		tempConn := *conn
		tempConn.Database = database

		createSQL, err := u.chClient.GetCreateSQL(ctx, &tempConn, tableName)
		if err != nil {
			// Continue with other tables if one fails
			createSQL = fmt.Sprintf("-- Failed to fetch CREATE TABLE for %s.%s: %v", database, tableName, err)
		}

		tableSchemas = append(tableSchemas, entity.TableSchemaInfo{
			Database:   database,
			TableName:  tableName,
			CreateSQL:  createSQL,
			HasWarning: hasWarning,
		})
	}

	// Generate analysis text
	analysisText := u.generateAnalysisText(query, explainPlan, queryStats, tableSchemas, warnings)

	return &entity.QueryAnalysis{
		Query:        query,
		Tables:       tableSchemas,
		ExplainPlan:  explainPlan,
		QueryStats:   queryStats,
		AnalysisText: analysisText,
		Warnings:     warnings,
	}, nil
}

func (u *ConnectionUsecase) extractTableNames(query string) []string {
	// Normalize query - remove comments and extra whitespace
	normalizedQuery := strings.ToUpper(query)

	// Common SQL patterns to extract table names
	patterns := []string{
		`FROM\s+([^\s,(]+)`,   // FROM table
		`JOIN\s+([^\s,(]+)`,   // JOIN table
		`INTO\s+([^\s,(]+)`,   // INSERT INTO table
		`UPDATE\s+([^\s,(]+)`, // UPDATE table
		`TABLE\s+([^\s,(]+)`,  // DROP TABLE, CREATE TABLE, etc.
	}

	tableNames := make([]string, 0)
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(normalizedQuery, -1)
		for _, match := range matches {
			if len(match) > 1 {
				tableName := strings.Trim(match[1], "`\"")
				// Skip subqueries and system tables
				if !strings.Contains(tableName, "(") &&
					!strings.HasPrefix(tableName, "SYSTEM.") &&
					!seen[tableName] &&
					tableName != "" {
					seen[tableName] = true
					tableNames = append(tableNames, tableName)
				}
			}
		}
	}

	return tableNames
}

type tableReference struct {
	Database  string
	TableName string
}

func (u *ConnectionUsecase) extractTableReferences(query string) []tableReference {
	// Common SQL patterns to extract table references (database.table or just table)
	patterns := []string{
		`FROM\s+([^\s,(]+)`,   // FROM table
		`JOIN\s+([^\s,(]+)`,   // JOIN table
		`INTO\s+([^\s,(]+)`,   // INSERT INTO table
		`UPDATE\s+([^\s,(]+)`, // UPDATE table
		`TABLE\s+([^\s,(]+)`,  // DROP TABLE, CREATE TABLE, etc.
	}

	references := make([]tableReference, 0)
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(query, -1) // Use original query for case sensitivity
		for _, match := range matches {
			if len(match) > 1 {
				fullRef := strings.Trim(match[1], "`\"")

				// Skip subqueries and system tables
				if strings.Contains(fullRef, "(") || strings.HasPrefix(strings.ToUpper(fullRef), "SYSTEM.") {
					continue
				}

				// Create unique key for deduplication
				uniqueKey := strings.ToUpper(fullRef)
				if seen[uniqueKey] {
					continue
				}
				seen[uniqueKey] = true

				// Split by dot to get database and table
				parts := strings.Split(fullRef, ".")
				var database, tableName string

				if len(parts) >= 2 {
					database = parts[0]
					tableName = strings.Join(parts[1:], ".") // Handle case like db.schema.table
				} else {
					tableName = fullRef
				}

				references = append(references, tableReference{
					Database:  database,
					TableName: tableName,
				})
			}
		}
	}

	return references
}

func (u *ConnectionUsecase) generateAnalysisText(query string, explainPlan string, queryStats *entity.QueryStats, tableSchemas []entity.TableSchemaInfo, warnings []string) string {
	var sb strings.Builder

	sb.WriteString("## Query Analysis\n\n")
	sb.WriteString("Please analyze the following ClickHouse query for performance optimization:\n\n")

	// Add warnings if any
	if len(warnings) > 0 {
		sb.WriteString("### ⚠️ Warnings\n\n")
		for _, warning := range warnings {
			sb.WriteString(fmt.Sprintf("- %s\n", warning))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("### Query\n")
	sb.WriteString("```sql\n")
	sb.WriteString(query)
	sb.WriteString("\n```\n\n")

	// Add EXPLAIN plan
	sb.WriteString("### EXPLAIN Plan\n")
	if explainPlan != "" && !strings.HasPrefix(explainPlan, "-- Failed") {
		sb.WriteString("```sql\n")
		sb.WriteString(explainPlan)
		sb.WriteString("```\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("%s\n\n", explainPlan))
	}

	// Add Query Execution Stats
	sb.WriteString("### Current Execution Stats\n")
	if queryStats != nil {
		sb.WriteString(fmt.Sprintf("- **Duration**: %d ms\n", queryStats.ExecutionTimeMs))
		sb.WriteString(fmt.Sprintf("- **Rows Read**: %s\n", formatNumber(uint64(queryStats.RowsRead))))
		sb.WriteString(fmt.Sprintf("- **Bytes Read**: %s\n", formatBytes(queryStats.BytesRead)))
		sb.WriteString(fmt.Sprintf("- **Memory Peak**: %s\n", formatBytes(queryStats.MemoryPeak)))
		sb.WriteString(fmt.Sprintf("- **Parts Read**: %s\n", formatNumber(queryStats.PartsRead)))
		sb.WriteString(fmt.Sprintf("- **Marks Read**: %s\n", formatNumber(queryStats.MarksRead)))
	} else {
		sb.WriteString("_Stats not available_\n")
	}
	sb.WriteString("\n")

	if len(tableSchemas) > 0 {
		sb.WriteString(fmt.Sprintf("### Table Schemas (%d tables)\n\n", len(tableSchemas)))
		for i, ts := range tableSchemas {
			if ts.Database != "" {
				sb.WriteString(fmt.Sprintf("#### %d. %s.%s", i+1, ts.Database, ts.TableName))
			} else {
				sb.WriteString(fmt.Sprintf("#### %d. %s", i+1, ts.TableName))
			}
			if ts.HasWarning {
				sb.WriteString(" ⚠️ *database not specified in query*")
			}
			sb.WriteString("\n")
			sb.WriteString("```sql\n")
			sb.WriteString(ts.CreateSQL)
			sb.WriteString("\n```\n\n")
		}
	}

	sb.WriteString("### Analysis Request\n\n")
	sb.WriteString("Please provide:\n")
	sb.WriteString("1. Performance optimization suggestions\n")
	sb.WriteString("2. Index recommendations (if applicable)\n")
	sb.WriteString("3. Query rewrite suggestions for better performance\n")
	sb.WriteString("4. Potential bottlenecks in the query\n")
	sb.WriteString("5. ClickHouse-specific optimizations\n")

	return sb.String()
}

// Helper function to format bytes
func formatBytes(bytes uint64) string {
	if bytes == 0 {
		return "0 B"
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Helper function to format numbers
func formatNumber(num uint64) string {
	s := fmt.Sprintf("%d", num)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "." + s[i:]
	}
	return s
}
