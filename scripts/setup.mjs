import { randomBytes } from "node:crypto";
import { existsSync, writeFileSync } from "node:fs";
if (existsSync(".env")) {
  console.log(".env already exists; retained existing configuration.");
} else {
  const secret = randomBytes(48).toString("hex");
  const password = randomBytes(24).toString("hex");
  writeFileSync(
    ".env",
    `APP_URL=http://localhost:8080\nAPP_SECRET=${secret}\nPOSTGRES_PASSWORD=${password}\nAPP_PORT=8080\n`,
    { mode: 0o600, flag: "wx" },
  );
  console.log("Created .env with random application and database secrets.");
}
