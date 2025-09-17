{
  "name": "typescript-simple-template",
  "version": "1.0.0",
  "main": "dist/main.js",
  "private": true,
  "scripts": {
    "build:all": "bun run ./node_modules/cre-sdk-typescript/scripts/build-for-cli.ts"
  },
  "license": "UNLICENSED",
  "dependencies": {
    "cre-sdk-typescript": "github:smartcontractkit/cre-sdk-typescript#18e7bbd"
  },
  "devDependencies": {
    "@types/bun": "1.2.21",
    "@types/node": "24.3.1"
  }
}
