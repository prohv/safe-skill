const fs = require("fs");

const map = {
  win32:  "bin/safeskill-win.exe",
  darwin: "bin/safeskill-darwin",
  linux:  "bin/safeskill-linux",
};

const src = map[process.platform];
if (!src) {
  console.error("Unsupported platform: " + process.platform);
  process.exit(1);
}

const dest = process.platform === "win32" ? "bin/safeskill.exe" : "bin/safeskill";
fs.copyFileSync(src, dest);
if (process.platform !== "win32") fs.chmodSync(dest, 0o755);

console.log("SafeSkill installed! Usage:");
console.log("  safe-skill scan <path>");
console.log("  safe-skill proxy wrap -- npm install <pkg>");
console.log("  safe-skill proxy run");
