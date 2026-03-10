---
name: query-analyzer
description: Skill for analyzing and optimizing ClickHouse queries in go-ch-manager.
---

# ClickHouse Query Analyzer Skill

This skill provides a structured workflow for analyzing and optimizing ClickHouse queries. When a user provides a query for analysis, follow these steps:

## Step 1: Schema Assessment
Review the table definition to understand the physical data layout.
- **Tools**: `SHOW CREATE TABLE {table}` or `DESCRIBE TABLE {table}`.
- **Checklist**:
    - Identify **Partition Key**: Is the query filtering by it?
    - Identify **Sorting Key (Primary Key)**: Is the query leveraging the prefix of the sorting key in `WHERE`/`PREWHERE`?
    - Check **Data Types**: Are there `LowCardinality` strings or `Nullable` columns that could be optimized?

## Step 2: Query Plan Analysis
Use ClickHouse's built-in `EXPLAIN` to see how the engine intends to execute the query.
- **Commands**:
    - `EXPLAIN indexes=1 {query}`: Check if any marks/parts are being skipped by indexes.
    - `EXPLAIN actions=1 {query}`: See the detailed execution steps.
    - `EXPLAIN PIPELINE {query}`: Check the level of parallelism.

## Step 3: Runtime Performance Audit
If the query has been run, analyze its actual resource consumption.
- **Source**: `system.query_log`.
- **Key Metrics to Inspect**:
    - `read_rows` vs `result_rows`: High ratio indicates inefficient filtering.
    - `read_bytes`: Total I/O overhead.
    - `memory_usage`: Peak memory consumed (crucial for large JOINs or Aggregations).
    - `query_duration_ms`: Total latency.

## Step 4: Common Optimization Strategies
Apply these patterns to improve performance:
1. **Leverage PREWHERE**: Move filters on primary key columns or small columns to `PREWHERE` to prune data before reading large columns.
2. **Avoid SELECT ***: Specify only necessary columns to minimize I/O in the columnar storage.
3. **Optimize Joins**: ClickHouse prefers `JOIN`s where the right-side table fits in memory. Consider using `Dictionaries` for high-performance lookups.
4. **Partition Pruning**: Ensure filters on partition keys (usually time-based) are present to avoid scanning all data parts.
5. **Function Pushdown**: Avoid wrapping columns in functions in the `WHERE` clause (e.g., use `date >= '2023-01-01'` instead of `toYear(date) = 2023`).

## Analysis Report Format
When providing your analysis, structure it as follows:
1. **Summary**: High-level assessment (e.g., "I/O bound", "Memory intensive").
2. **Schema Audit**: Insights from table definition.
3. **Execution Plan**: Insights from `EXPLAIN`.
4. **Bottlenecks**: Specific causes of slowness.
5. **Recommendations**: Numbered list of actionable SQL changes or schema improvements.
