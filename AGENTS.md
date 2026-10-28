Before implementing any changes, use Graphify to understand the relevant parts of the existing codebase.

1. Query the knowledge graph to identify relevant modules, classes, functions, dependencies, and call paths.
2. Trace how the proposed changes will affect existing components and data flows.
3. Read the relevant source files to verify the graph's findings and understand implementation details.
4. Implement the changes consistently with the existing architecture and coding conventions.
5. Check for regressions, broken imports, outdated references, and affected tests.
6. Update the Graphify knowledge graph after making significant code changes, if necessary.

Do not explore the entire repository unnecessarily. Use targeted graph queries to understand the components relevant to the task.

If Graphify is unavailable or the graph is outdated, say so and use the source code directly rather than pretending to have queried the graph.