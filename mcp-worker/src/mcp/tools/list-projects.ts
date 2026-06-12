import { z } from "zod";
import { defineTool } from "../../lib/define-tool";

// Mirrors ListProjects in db/project.go.
export default defineTool({
  name: "list_projects",
  description:
    "List tq projects, newest first. Each project groups tasks and actions.",
  inputSchema: {
    limit: z
      .number()
      .int()
      .min(1)
      .max(100)
      .default(20)
      .describe("Maximum number of projects to return."),
  },
  annotations: { readOnlyHint: true },
  async handler(params, { db }) {
    const result = await db.execute({
      sql: "SELECT id, name, work_dir, metadata, dispatch_enabled, created_at FROM projects ORDER BY id DESC LIMIT ?",
      args: [params.limit],
    });
    if (result.rows.length === 0) {
      return "No projects found.";
    }
    const lines = result.rows.map((row) => {
      const workDir = row.work_dir === "" ? "-" : String(row.work_dir);
      const dispatch = Number(row.dispatch_enabled) === 1 ? "on" : "off";
      return `- #${row.id} **${row.name}** (work_dir: ${workDir}, dispatch: ${dispatch}, created: ${row.created_at})`;
    });
    return `# tq projects\n\n${lines.join("\n")}`;
  },
});
