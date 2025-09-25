{
  "compilerOptions": {
    "target": "es2016",
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
    "main.ts"
  ]
}
