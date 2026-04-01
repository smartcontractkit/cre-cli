## cre login

Start authentication flow

### Synopsis

Opens a browser for interactive login and saves credentials.

For non-interactive environments (CI/CD, automation, AI agents), set the
CRE_API_KEY environment variable instead:

  export CRE_API_KEY=<your-api-key>

API keys can be created at https://app.chain.link (see Account Settings).
When CRE_API_KEY is set, all commands that require authentication will use
it automatically — no login needed.

```
cre login [optional flags]
```

### Options

```
  -h, --help              help for login
      --non-interactive   Fail instead of prompting; requires all inputs via flags
```

### Options inherited from parent commands

```
  -e, --env string            Path to .env file which contains sensitive info
  -R, --project-root string   Path to the project root
  -E, --public-env string     Path to .env.public file which contains shared, non-sensitive build config
  -T, --target string         Use target settings from YAML config
  -v, --verbose               Run command in VERBOSE mode
```

### SEE ALSO

* [cre](cre.md)	 - CRE CLI tool

