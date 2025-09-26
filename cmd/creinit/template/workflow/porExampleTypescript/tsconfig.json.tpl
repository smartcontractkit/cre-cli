{
  "compilerOptions": {
    "target": "esnext",
    "module": "commonjs",
    "outDir": "./dist",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
     "paths": {
      // TODO: Remove this when the npm package lands
      "@chainlink/cre-sdk/*": ["./node_modules/cre-sdk-typescript/src/sdk/*"]
    }
  },
  "include": [
    "global.d.ts",
    "main.ts"
  ]
}
