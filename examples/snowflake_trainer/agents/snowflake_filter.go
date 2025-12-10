package agents

import (
	"context"
	"fmt"
	"strings"

	"github.com/assagman/dsgo"
)

// SnowflakeDomainFilter wraps MCP tools to enforce Snowflake domain restrictions
type SnowflakeDomainFilter struct {
	tools []dsgo.Tool
}

// NewSnowflakeDomainFilter creates a filter that restricts searches to Snowflake domains
func NewSnowflakeDomainFilter(tools []dsgo.Tool) *SnowflakeDomainFilter {
	return &SnowflakeDomainFilter{
		tools: tools,
	}
}

// GetFilteredTools returns tools with Snowflake domain restrictions applied
func (f *SnowflakeDomainFilter) GetFilteredTools() []dsgo.Tool {
	var filtered []dsgo.Tool

	for _, tool := range f.tools {
		// Only wrap web search tools, not code context tools
		if tool.Name == "web_search_exa" {
			filtered = append(filtered, f.wrapWebSearchTool(tool))
		} else {
			filtered = append(filtered, tool)
		}
	}

	return filtered
}

// wrapWebSearchTool creates a wrapper that adds domain restrictions to search queries
func (f *SnowflakeDomainFilter) wrapWebSearchTool(originalTool dsgo.Tool) dsgo.Tool {
	// Create a new tool with the same metadata but wrapped function
	wrappedFunc := func(ctx context.Context, args map[string]any) (any, error) {
		// Add domain restrictions to the query
		if query, ok := args["query"].(string); ok {
			// Append Snowflake domain restrictions
			domains := []string{
				"site:snowflake.com",
				"site:docs.snowflake.com",
				"site:community.snowflake.com",
				"site:snowflakecommunity.force.com",
			}

			// Check if query already has site: restrictions
			hasSiteRestriction := strings.Contains(query, "site:")

			if !hasSiteRestriction {
				// Add domain restrictions
				restrictedQuery := fmt.Sprintf("%s (%s)", query, strings.Join(domains, " OR "))
				args["query"] = restrictedQuery
			}
		}

		// Call the original tool
		return originalTool.Function(ctx, args)
	}

	// Create new tool with wrapped function
	wrappedTool := dsgo.NewTool(
		originalTool.Name,
		originalTool.Description+" (Snowflake domain restricted)",
		wrappedFunc,
	)

	// Copy parameters
	for _, param := range originalTool.Parameters {
		switch param.Type {
		case "array":
			wrappedTool.AddArrayParameter(param.Name, param.Description, param.ElementType, param.Required)
		case "json":
			wrappedTool.AddParameter(param.Name, "json", param.Description, param.Required)
		default:
			wrappedTool.AddParameter(param.Name, param.Type, param.Description, param.Required)
		}
	}

	return *wrappedTool
}

// AddSnowflakeContext enhances sub-topics with Snowflake-specific context
func AddSnowflakeContext(subTopics []string, skillLevel string) []map[string]any {
	var researchInputs []map[string]any

	for _, subTopic := range subTopics {
		// Enhance sub-topic with Snowflake context
		enhancedSubTopic := fmt.Sprintf("Snowflake %s", subTopic)

		researchInputs = append(researchInputs, map[string]any{
			"subTopic":     enhancedSubTopic,
			"learningGoal": fmt.Sprintf("Understand Snowflake %s for %s level. Focus exclusively on official Snowflake documentation, best practices, and platform-specific features. Only use sources from snowflake.com, docs.snowflake.com, or community.snowflake.com.", subTopic, skillLevel),
			"skillLevel":   skillLevel,
			"platform":     "Snowflake Data Cloud Platform",
			"domains":      "snowflake.com,docs.snowflake.com,community.snowflake.com",
		})
	}

	return researchInputs
}

// ValidateSnowflakeContent checks if research output contains Snowflake-specific content
func ValidateSnowflakeContent(content string) bool {
	snowflakeIndicators := []string{
		"Snowflake",
		"snowflake",
		"SNOWFLAKE",
		"docs.snowflake.com",
		"community.snowflake.com",
		"SnowSQL",
		"Snowpipe",
		"Snowpark",
		"Virtual Warehouse",
		"Time Travel",
		"Fail-safe",
		"Zero-copy cloning",
		"External stages",
		"Internal stages",
		"File formats",
		"COPY INTO",
		"Snowsight",
		"Account usage",
		"Information schema",
		"Query profile",
		"Warehouse sizing",
		"Multi-cluster",
		"Auto-scaling",
		"Resource monitors",
		"Storage integration",
		"External functions",
		"UDFs",
		"Stored procedures",
		"Tasks",
		"Streams",
		"Materialized views",
		"Secure views",
		"Dynamic data masking",
		"Row access policies",
		"Data sharing",
		"Snowflake Marketplace",
		"External tables",
		"Iceberg tables",
		"Hybrid tables",
		"Event tables",
		"Change tracking",
		"Search optimization",
		"Clustering keys",
		"Automatic clustering",
		"Query acceleration",
		"Materialized views",
		"Result caching",
		"Metadata caching",
		"Data retention",
		"Time travel",
		"Fail-safe",
		"Data governance",
		"Column-level security",
		"Row-level security",
		"Tag-based masking",
		"Object tagging",
		"Access history",
		"Login history",
		"Query history",
		"COPY history",
		"Load history",
		"Snowpipe history",
		"Replication",
		"Database replication",
		"Account replication",
		"Failover groups",
		"Business continuity",
		"Disaster recovery",
		"Snowflake Cortex",
		"Snowflake ML",
		"Snowpark ML",
		"Feature store",
		"Model registry",
		"Inference API",
		"Snowflake Native Apps",
		"Snowflake CLI",
		"SnowSQL CLI",
		"Snowsight UI",
		"Classic console",
		"Account admin",
		"Security admin",
		"User admin",
		"System admin",
		"Public schema",
		"Database roles",
		"Schema-level privileges",
		"Future grants",
		"Managed access",
		"Network policies",
		"PrivateLink",
		"Snowgrid",
		"Snowpipe streaming",
		"Kafka connector",
		"Snowflake Connector",
		"ODBC driver",
		"JDBC driver",
		"Python connector",
		"Node.js driver",
		"Go driver",
		".NET driver",
		"PHP driver",
		"Snowflake Partner Connect",
		"Partner solutions",
		"Technology partners",
		"SI partners",
		"Snowflake credits",
		"Warehouse credits",
		"Cloud services credits",
		"Storage costs",
		"Data transfer",
		"Compute costs",
		"Auto-suspend",
		"Auto-resume",
		"Min/max clusters",
		"Scaling policy",
		"Statement timeout",
		"Query timeout",
		"Concurrency scaling",
		"Max concurrency level",
		"Memory limits",
		"Spilling",
		"Remote disk",
		"Local disk",
		"Result set",
		"Query ID",
		"Query tag",
		"Session parameters",
		"Account parameters",
		"Object parameters",
		"WAREHOUSE_SIZE",
		"WAREHOUSE_TYPE",
		"AUTO_SUSPEND",
		"AUTO_RESUME",
		"MIN_CLUSTER_COUNT",
		"MAX_CLUSTER_COUNT",
		"SCALING_POLICY",
		"STATEMENT_TIMEOUT_IN_SECONDS",
		"QUERY_TAG",
		"TIMEZONE",
		"DATE_FORMAT",
		"TIMESTAMP_FORMAT",
		"TIMESTAMP_LTZ_OUTPUT_FORMAT",
		"TIMESTAMP_NTZ_OUTPUT_FORMAT",
		"TIMESTAMP_TZ_OUTPUT_FORMAT",
		"BINARY_OUTPUT_FORMAT",
		"JSON_INDENT",
		"LOCK_TIMEOUT",
		"STATEMENT_QUEUED_TIMEOUT_IN_SECONDS",
		"STATEMENT_TIMEOUT_IN_SECONDS",
		"TRANSACTION_TIMEOUT_IN_SECONDS",
		"WEEK_START",
		"WEEK_OF_YEAR_POLICY",
		"JDBC_TREAT_DECIMAL_AS_INT",
		"JDBC_TREAT_TIMESTAMP_NTZ_AS_UTC",
		"JDBC_USE_SESSION_TIMEZONE",
		"ODBC_TREAT_DECIMAL_AS_INT",
		"ENABLE_UNLOAD_PHYSICAL_TYPE_OPTIMIZATION",
		"ENABLE_UNLOAD_PHYSICAL_TYPE_OPTIMIZATION",
		"ENABLE_UNLOAD_PHYSICAL_TYPE_OPTIMIZATION",
	}

	contentLower := strings.ToLower(content)
	for _, indicator := range snowflakeIndicators {
		if strings.Contains(contentLower, strings.ToLower(indicator)) {
			return true
		}
	}

	return false
}
