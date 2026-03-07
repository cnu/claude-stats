package nlquery

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cnu/claude-stats/internal/db"
)

// Pattern maps a natural language regex to a SQL query.
type Pattern struct {
	Regex       *regexp.Regexp
	SQL         string
	Description string
}

// Engine matches natural language questions to SQL queries.
type Engine struct {
	patterns []Pattern
	database *db.DB
}

// New creates a new NL query engine.
func New(database *db.DB) *Engine {
	return &Engine{
		patterns: defaultPatterns(),
		database: database,
	}
}

// Query matches the input against known patterns and executes the resulting SQL.
// Returns the query result, the generated SQL, and any error.
func (e *Engine) Query(input string) (*db.QueryResult, string, error) {
	normalized := strings.ToLower(strings.TrimSpace(input))

	for _, p := range e.patterns {
		matches := p.Regex.FindStringSubmatch(normalized)
		if matches == nil {
			continue
		}

		sql := p.SQL
		// Replace captured groups as $1, $2, etc.
		for i := 1; i < len(matches); i++ {
			sql = strings.ReplaceAll(sql, fmt.Sprintf("$%d", i), matches[i])
		}

		result, err := e.database.ExecuteQuery(sql, 50)
		if err != nil {
			return nil, sql, fmt.Errorf("execute query: %w", err)
		}
		return result, sql, nil
	}

	return nil, "", fmt.Errorf("unknown query, try:\n" +
		"  total cost\n" +
		"  cost today / this week / this month\n" +
		"  how many sessions\n" +
		"  most expensive session\n" +
		"  top 5 models\n" +
		"  cost by project\n" +
		"  top tools\n" +
		"  busiest day\n" +
		"  cost for <project-name>")
}

// Examples returns a list of example queries for display.
func Examples() []string {
	return []string{
		"total cost",
		"cost today",
		"cost this week",
		"how many sessions",
		"most expensive session",
		"top 5 models",
		"cost by project",
		"top tools",
		"busiest day",
		"longest session",
		"average session cost",
		"daily cost",
	}
}
