package nlquery

import "regexp"

func defaultPatterns() []Pattern {
	return []Pattern{
		{
			Regex:       regexp.MustCompile(`(?:what(?:'s| is) (?:the |my )?)?total cost`),
			Description: "Total cost across all sessions",
			SQL:         `SELECT printf('$%.2f', COALESCE(SUM(total_cost_usd), 0)) AS total_cost FROM sessions`,
		},
		{
			Regex:       regexp.MustCompile(`cost today`),
			Description: "Cost for today",
			SQL:         `SELECT date_key AS date, printf('$%.2f', total_cost_usd) AS cost, session_count AS sessions, message_count AS messages FROM daily_stats WHERE date_key = date('now', 'localtime')`,
		},
		{
			Regex:       regexp.MustCompile(`cost yesterday`),
			Description: "Cost for yesterday",
			SQL:         `SELECT date_key AS date, printf('$%.2f', total_cost_usd) AS cost, session_count AS sessions, message_count AS messages FROM daily_stats WHERE date_key = date('now', 'localtime', '-1 day')`,
		},
		{
			Regex:       regexp.MustCompile(`cost (?:this |current )?week`),
			Description: "Cost for the current week",
			SQL:         `SELECT printf('$%.2f', COALESCE(SUM(total_cost_usd), 0)) AS cost, COALESCE(SUM(session_count), 0) AS sessions, COALESCE(SUM(message_count), 0) AS messages FROM daily_stats WHERE date_key >= date('now', 'localtime', 'weekday 0', '-6 days')`,
		},
		{
			Regex:       regexp.MustCompile(`cost (?:this |current )?month`),
			Description: "Cost for the current month",
			SQL:         `SELECT printf('$%.2f', COALESCE(SUM(total_cost_usd), 0)) AS cost, COALESCE(SUM(session_count), 0) AS sessions, COALESCE(SUM(message_count), 0) AS messages FROM daily_stats WHERE date_key >= date('now', 'localtime', 'start of month')`,
		},
		{
			Regex:       regexp.MustCompile(`cost (?:last |past )?(\d+) days?`),
			Description: "Cost for the last N days",
			SQL:         `SELECT printf('$%.2f', COALESCE(SUM(total_cost_usd), 0)) AS cost, COALESCE(SUM(session_count), 0) AS sessions, COALESCE(SUM(message_count), 0) AS messages FROM daily_stats WHERE date_key >= date('now', 'localtime', '-$1 days')`,
		},
		{
			Regex:       regexp.MustCompile(`most expensive session`),
			Description: "The most expensive session",
			SQL: `SELECT COALESCE(project_name, '(unknown)') AS project,
				printf('$%.4f', total_cost_usd) AS cost,
				message_count AS messages,
				datetime(first_message_at/1000, 'unixepoch', 'localtime') AS started
				FROM sessions ORDER BY total_cost_usd DESC LIMIT 1`,
		},
		{
			Regex:       regexp.MustCompile(`how many sessions`),
			Description: "Total session count",
			SQL:         `SELECT COUNT(*) AS total_sessions FROM sessions`,
		},
		{
			Regex:       regexp.MustCompile(`how many messages`),
			Description: "Total message count",
			SQL:         `SELECT COUNT(*) AS total_messages FROM messages`,
		},
		{
			Regex:       regexp.MustCompile(`top (\d+) models?`),
			Description: "Top N models by usage",
			SQL: `SELECT model, COUNT(*) AS messages, printf('$%.2f', SUM(cost_usd)) AS cost
				FROM messages WHERE model != '' GROUP BY model ORDER BY COUNT(*) DESC LIMIT $1`,
		},
		{
			Regex:       regexp.MustCompile(`top models?`),
			Description: "Top models by usage",
			SQL: `SELECT model, COUNT(*) AS messages, printf('$%.2f', SUM(cost_usd)) AS cost
				FROM messages WHERE model != '' GROUP BY model ORDER BY COUNT(*) DESC LIMIT 10`,
		},
		{
			Regex:       regexp.MustCompile(`cost (?:by|per) project`),
			Description: "Cost breakdown by project",
			SQL: `SELECT COALESCE(project_name, '(unknown)') AS project,
				printf('$%.2f', COALESCE(SUM(total_cost_usd), 0)) AS cost,
				COUNT(*) AS sessions
				FROM sessions GROUP BY project_name ORDER BY SUM(total_cost_usd) DESC`,
		},
		{
			Regex:       regexp.MustCompile(`sessions? today`),
			Description: "Sessions from today",
			SQL: `SELECT COALESCE(project_name, '(unknown)') AS project,
				message_count AS messages,
				printf('$%.4f', total_cost_usd) AS cost,
				datetime(first_message_at/1000, 'unixepoch', 'localtime') AS started
				FROM sessions
				WHERE date(first_message_at/1000, 'unixepoch', 'localtime') = date('now', 'localtime')
				ORDER BY first_message_at DESC`,
		},
		{
			Regex:       regexp.MustCompile(`sessions? (?:this |current )?week`),
			Description: "Sessions from this week",
			SQL: `SELECT COALESCE(project_name, '(unknown)') AS project,
				message_count AS messages,
				printf('$%.4f', total_cost_usd) AS cost,
				datetime(first_message_at/1000, 'unixepoch', 'localtime') AS started
				FROM sessions
				WHERE date(first_message_at/1000, 'unixepoch', 'localtime') >= date('now', 'localtime', 'weekday 0', '-6 days')
				ORDER BY first_message_at DESC`,
		},
		{
			Regex:       regexp.MustCompile(`average (?:session )?cost|cost per session`),
			Description: "Average cost per session",
			SQL:         `SELECT printf('$%.4f', AVG(total_cost_usd)) AS avg_cost, COUNT(*) AS sessions FROM sessions`,
		},
		{
			Regex:       regexp.MustCompile(`longest session`),
			Description: "The longest session by duration",
			SQL: `SELECT COALESCE(project_name, '(unknown)') AS project,
				printf('$%.4f', total_cost_usd) AS cost,
				message_count AS messages,
				CASE
					WHEN total_duration_ms >= 3600000 THEN printf('%dh%dm', total_duration_ms/3600000, (total_duration_ms%3600000)/60000)
					WHEN total_duration_ms >= 60000 THEN printf('%dm%ds', total_duration_ms/60000, (total_duration_ms%60000)/1000)
					ELSE printf('%ds', total_duration_ms/1000)
				END AS duration,
				datetime(first_message_at/1000, 'unixepoch', 'localtime') AS started
				FROM sessions ORDER BY total_duration_ms DESC LIMIT 1`,
		},
		{
			Regex:       regexp.MustCompile(`(?:top|most (?:used|common)) tools?`),
			Description: "Most used tools",
			SQL:         `SELECT tool_name, COUNT(*) AS uses FROM tool_uses GROUP BY tool_name ORDER BY uses DESC LIMIT 15`,
		},
		{
			Regex:       regexp.MustCompile(`cost (?:for|of) (.+)`),
			Description: "Cost for a specific project",
			SQL: `SELECT COALESCE(project_name, '(unknown)') AS project,
				printf('$%.2f', COALESCE(SUM(total_cost_usd), 0)) AS cost,
				COUNT(*) AS sessions,
				SUM(message_count) AS messages
				FROM sessions WHERE project_name LIKE '%$1%' GROUP BY project_name`,
		},
		{
			Regex:       regexp.MustCompile(`busiest day`),
			Description: "The day with most activity",
			SQL:         `SELECT date_key AS date, message_count AS messages, session_count AS sessions, printf('$%.2f', total_cost_usd) AS cost FROM daily_stats ORDER BY message_count DESC LIMIT 1`,
		},
		{
			Regex:       regexp.MustCompile(`daily cost`),
			Description: "Daily cost for the last 14 days",
			SQL:         `SELECT date_key AS date, printf('$%.2f', total_cost_usd) AS cost, session_count AS sessions, message_count AS messages FROM daily_stats ORDER BY date_key DESC LIMIT 14`,
		},
	}
}
