# VigilAfrica Web

React 19 + Vite frontend for VigilAfrica.

## Environment Mapping

| Environment | Branch | Vercel project | Domain | API base URL |
|---|---|---|---|---|
| Local | any | local dev server | `http://localhost:5173` | `http://localhost:8080` |
| Staging | `main` | `vigilafrica-staging` | `https://staging.vigilafrica.org` | `https://api.staging.vigilafrica.org` |
| Production | `release` | `vigilafrica-production` | `https://vigilafrica.org` | `https://api.vigilafrica.org` |

Set `VITE_API_BASE_URL` in each Vercel project. Do not add secrets to any `VITE_` variable; Vite exposes those values in the browser bundle.

## Local Development

```bash
npm install
npm run dev
```

## Verification

```bash
npm run test
npm run lint
npm run build
```
