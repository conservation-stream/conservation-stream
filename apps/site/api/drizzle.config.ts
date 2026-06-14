import { defineConfig } from "drizzle-kit";

export default defineConfig({
  dialect: "postgresql",
  schema: "./src/server/db/schema/index.ts",
  dbCredentials: {
    url: "postgresql://user:password@localhost:5432/db"
  },
});
