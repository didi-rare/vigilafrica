# Changelog

## [1.3.1](https://github.com/didi-rare/vigilafrica/compare/v1.3.0...v1.3.1) (2026-07-18)


### Bug Fixes

* **compose:** probe umami healthcheck over IPv4 (127.0.0.1) ([b6244fb](https://github.com/didi-rare/vigilafrica/commit/b6244fbab349501c25efcf5c955aa8b3cf4b9c76))
* **compose:** probe umami healthcheck over IPv4 on release (bundle into v1.3.1) ([cdfdadd](https://github.com/didi-rare/vigilafrica/commit/cdfdadd602e17a03be48e01ffb0c917bb3aa60c1))
* **csp:** allow Umami tracker origin in script-src ([5c35e84](https://github.com/didi-rare/vigilafrica/commit/5c35e845dd3dec6d4976988c53d44e9cb1d0a82c))
* **csp:** allow Umami tracker origin in script-src on release (fixes prod analytics) ([4a76674](https://github.com/didi-rare/vigilafrica/commit/4a76674aef4c7c15cb7566bfd9bb861c61139839))

## [1.3.0](https://github.com/didi-rare/vigilafrica/compare/v1.2.0...v1.3.0) (2026-07-17)


### Features

* **analytics:** add self-hosted Umami infra for dev/staging/prod (Day 1) ([64180f7](https://github.com/didi-rare/vigilafrica/commit/64180f77964625a9b1c322db5ffba5cef056b851))
* **analytics:** self-hosted Umami + 1-click feedback widget (chore-analytics-and-feedback) ([79ffb03](https://github.com/didi-rare/vigilafrica/commit/79ffb03a4108727d071c54578b0a5094213a7ba2))
* **analytics:** wire frontend tracker, custom events, and feedback widget (Day 2) ([e007596](https://github.com/didi-rare/vigilafrica/commit/e0075961360839b777b7688321475599f42f2218))
* **digest:** daily flood digest endpoint + scheduled email ([6374b55](https://github.com/didi-rare/vigilafrica/commit/6374b55a7499527e8f0583c9f7a2b6a0cfc7de24))
* **digest:** daily flood digest endpoint + scheduled email (feature-daily-flood-digest) ([93d7d8c](https://github.com/didi-rare/vigilafrica/commit/93d7d8c00bd58108820c9b12251bd22748485cab))


### Bug Fixes

* **analytics:** pin Umami image to digest to satisfy CI image-pin gate ([763ef37](https://github.com/didi-rare/vigilafrica/commit/763ef37cbb7fdf069a05e48532112bf8bfc27c72))
* **ci:** bump Go toolchain to 1.26.4 (June 2026 stdlib CVE batch) ([c40c249](https://github.com/didi-rare/vigilafrica/commit/c40c2496e603b13acab2e5936f4c8222fb5f83b5))
* **ci:** bump Go toolchain to 1.26.4 (stdlib CVE batch) ([00474a0](https://github.com/didi-rare/vigilafrica/commit/00474a04a31c3c4e583f8e188dcc1d4bec258972))
* **digest:** address openspec-review findings ([3d51621](https://github.com/didi-rare/vigilafrica/commit/3d51621a3f9afcb4aeb51ee5aa4850a2cad19fb1))
* **security:** bump Go to 1.26.5 + pgx to v5.9.2 (July 2026 govulncheck findings) ([e9c935b](https://github.com/didi-rare/vigilafrica/commit/e9c935b728af6cef481513c11bf2b67af1e66721))
* **security:** unblock the production cut — Go 1.26.5 + pgx 5.9.2 + npm audit fixes for main ([efdcb0c](https://github.com/didi-rare/vigilafrica/commit/efdcb0c30ea820b6f1f0ff9fd90542c205e20341))
* **web:** resolve npm audit advisories on main (vite, undici, js-yaml, babel) ([f734fb8](https://github.com/didi-rare/vigilafrica/commit/f734fb8ecb5a0d033b195ab9cb9a22099805e158))

## [1.2.0](https://github.com/didi-rare/vigilafrica/compare/v1.1.1...v1.2.0) (2026-05-26)


### Features

* **staging:** add stripe + icon + pulse to staging banner ([340a944](https://github.com/didi-rare/vigilafrica/commit/340a94411365bd2018242cc414507a6a506102b2))
* **staging:** add stripe + icon + pulse to staging banner ([3e69a15](https://github.com/didi-rare/vigilafrica/commit/3e69a1542072e676cb8be09add094c6a3a522c4a))


### Bug Fixes

* **api:** accept country_code alongside country; 400 on unknown values ([e674e7e](https://github.com/didi-rare/vigilafrica/commit/e674e7ee124c1c99ed26ea3847ca3cd5603c5914))
* **api:** accept country_code alongside country; 400 on unknown values ([a0c607f](https://github.com/didi-rare/vigilafrica/commit/a0c607f23f69b1ad94e57221315ba00de61f0af3))
* **api:** sync country_code openapi additions into the source-of-truth file ([e069f9e](https://github.com/didi-rare/vigilafrica/commit/e069f9e4ed189d85218103af84e236ac1afc2e88))
* **deps:** bump nested brace-expansion 5.0.5 → 5.0.6 ([ea76c8b](https://github.com/didi-rare/vigilafrica/commit/ea76c8bd5203606e4f66a3d8a6abeb6bdef831f9))
* **staging-banner:** apply review-round-1 polish (O1 + O2) ([a3fc298](https://github.com/didi-rare/vigilafrica/commit/a3fc298b577228060622eb321bd7815654e371ea))
* **staging:** scope VITE_ENV via Vercel dashboard + document the chain ([46f1a99](https://github.com/didi-rare/vigilafrica/commit/46f1a9916dc68e7331fd14381d8e2033ea2469e4))
* **staging:** scope VITE_ENV via Vercel dashboard + document the chain ([b8c4444](https://github.com/didi-rare/vigilafrica/commit/b8c4444d5959ac2c0773cc0f0d3c0a7022eb1ed1))

## [1.1.1](https://github.com/didi-rare/vigilafrica/compare/v1.1.0...v1.1.1) (2026-05-14)


### Bug Fixes

* **web:** cover unknown freshness state + queue css-tokens follow-up ([fef7846](https://github.com/didi-rare/vigilafrica/commit/fef7846a9cfba239d2de384c7844efcce529b5c1))
* **web:** public trust quick wins (banners, CTAs, OG meta, freshness) ([0b89e3a](https://github.com/didi-rare/vigilafrica/commit/0b89e3a5f963d6d1f738b6259c79457c0cf38888))
* **web:** public trust quick wins (banners, CTAs, OG meta, freshness) ([f950fb3](https://github.com/didi-rare/vigilafrica/commit/f950fb356ca2f0579e64a92b0300cf73fa5d9a32))

## [1.1.0](https://github.com/didi-rare/vigilafrica/compare/v1.0.1...v1.1.0) (2026-05-11)


### Features

* **ci:** scaffold release-please automation (dry-run) ([0946216](https://github.com/didi-rare/vigilafrica/commit/0946216da6ddd70e969f806c1f7807c4df2d93c0))
* **ci:** scaffold release-please automation (dry-run) ([eabfaba](https://github.com/didi-rare/vigilafrica/commit/eabfaba5186e20439307beb05d9571333af591a6))


### Bug Fixes

* **ci:** drop labels from cascade gh pr create ([ecefecd](https://github.com/didi-rare/vigilafrica/commit/ecefecda7c18cf0dbefdb8b4d8d003caff95f738))
* **ci:** pass target-branch explicitly to release-please-action ([a6fbf44](https://github.com/didi-rare/vigilafrica/commit/a6fbf444d581e858e5ff7f1e65a963cfaf679d9e))
* **ci:** pin release-please target-branch and revert orphaned 1.1.0 ([6b2a2ad](https://github.com/didi-rare/vigilafrica/commit/6b2a2adc9a6c9bf8d80cb2df9745528888cd81fd))
* **ci:** scope Vercel production deploys to release branch ([2662533](https://github.com/didi-rare/vigilafrica/commit/26625336f07cf84e90ebaaec1cd13d061ff5ae92))
* **ci:** scope Vercel production deploys to release branch via ignore script ([a70fc40](https://github.com/didi-rare/vigilafrica/commit/a70fc40e8d7a893015508f559f5e2b3f4a61a7d0))
* **ci:** simplify cascade-back-merge to use gh pr create ([18e78da](https://github.com/didi-rare/vigilafrica/commit/18e78daaf8c0d9ec0552ef61d83516b8573fe790))
* **ci:** use ref form for tag checkout in production deploy ([bdc12dc](https://github.com/didi-rare/vigilafrica/commit/bdc12dc10fd01179465a2ba2d4bc2c68f296e4ed))
