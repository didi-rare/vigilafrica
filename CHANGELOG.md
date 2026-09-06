# Changelog

## [1.6.0](https://github.com/didi-rare/vigilafrica/compare/v1.5.0...v1.6.0) (2026-09-04)


### Features

* **api:** narrow the trusted-proxy CIDRs to the measured gateway ([41921f0](https://github.com/didi-rare/vigilafrica/commit/41921f02f95c8b383966ae188ffb5cb757ad1e7d))
* **api:** narrow the trusted-proxy CIDRs to the measured gateway ([ff05cbd](https://github.com/didi-rare/vigilafrica/commit/ff05cbdde8ada0a82b9e8acf08b1329177ba5d06))
* **deploy:** split the deploy account per environment and harden sshd ([48d0b09](https://github.com/didi-rare/vigilafrica/commit/48d0b092d0992310df0fbccc610032abcd1db915))
* **deploy:** split the deploy account per environment and harden sshd ([7f143be](https://github.com/didi-rare/vigilafrica/commit/7f143bed4a575e74deec00287fed3530fb2b21c9))


### Bug Fixes

* **api:** drop lib/pq by moving migrations onto the pgx driver ([1a33296](https://github.com/didi-rare/vigilafrica/commit/1a33296c216a2bba4a53d48e9302a55835941c35))
* **api:** drop lib/pq by moving migrations onto the pgx driver ([8c9188f](https://github.com/didi-rare/vigilafrica/commit/8c9188f06455990ebed53d6870962cdc59d30980))

## [1.5.0](https://github.com/didi-rare/vigilafrica/compare/v1.4.0...v1.5.0) (2026-08-16)


### Features

* **deploy:** add a focused migration script for the forced-command cutover ([6e006ef](https://github.com/didi-rare/vigilafrica/commit/6e006efd040a2ef4630775cd0a74455fdefaa38a))
* **deploy:** forced-command deploy protocol and privilege boundary (1.3 + 1.4) ([62ea67c](https://github.com/didi-rare/vigilafrica/commit/62ea67c0e7ee7214ffbd7df87067de3eb545750b))
* **deploy:** forced-command deploy protocol and privilege boundary (tasks 1.3 + 1.4) ([e43b646](https://github.com/didi-rare/vigilafrica/commit/e43b646f5b35a4c08ba11458538141e219d0e950))


### Bug Fixes

* **ci:** bump Go toolchain to 1.26.6 to clear 7 reachable stdlib advisories ([d1d0977](https://github.com/didi-rare/vigilafrica/commit/d1d0977708932eed413ffdb60f133adf09466a53))
* **ci:** bump Go toolchain to 1.26.6 to clear 7 reachable stdlib advisories ([00a673c](https://github.com/didi-rare/vigilafrica/commit/00a673cfb0f98de60fe4c7388b6a4f7d618cebe1))
* **ci:** correct a false "honest limitation" and close the wiring gap ([287f0ef](https://github.com/didi-rare/vigilafrica/commit/287f0ef4b5b20c691754b08637d2eb083d5986a3))
* **ci:** correct host-key enrollment and validate the pinned known_hosts ([416980c](https://github.com/didi-rare/vigilafrica/commit/416980cc329eb561b382febc3d7d4a8f81dfe20d))
* **ci:** pin the VPS host key instead of relearning it every deploy ([e40f661](https://github.com/didi-rare/vigilafrica/commit/e40f661fd2a776d5c8d67767261c18438e3bf440))
* **ci:** pin the VPS host key instead of relearning it every deploy ([5b77225](https://github.com/didi-rare/vigilafrica/commit/5b772259ff8343b042cc1e80a60c340c5d68c0b4))
* **ci:** strengthen known_hosts validation after round-2 review ([59c4e51](https://github.com/didi-rare/vigilafrica/commit/59c4e51f7fed6e3daebd76f3b34ee9f9abfa4714))
* **deploy:** guard the destructive test harness (P0) ([8deebc2](https://github.com/didi-rare/vigilafrica/commit/8deebc2feefa539a3c3c1cf1c9f035b2b5a86e87))
* **deploy:** ownership migration was a silent no-op; fix rollout order and locking ([af83234](https://github.com/didi-rare/vigilafrica/commit/af8323440f301be6e64764216cb8298a96de6f14))
* **deploy:** re-clone instead of re-owning; close all round-1 migration findings ([e733262](https://github.com/didi-rare/vigilafrica/commit/e73326207187d90fe360a9028d288e86bc966513))
* **deploy:** sudoers wildcard did not restrict argv — use regex matching ([53a3071](https://github.com/didi-rare/vigilafrica/commit/53a3071e51b5f4efef9513c912adecdb89cd9f12))
* **deploy:** vigil-deploy-run swallowed every error message ([f0d24db](https://github.com/didi-rare/vigilafrica/commit/f0d24db6b151c5159e452ccd54b1c2bbb66b0517))
* **deploy:** vigil-deploy-run swallowed every error message ([b0399f0](https://github.com/didi-rare/vigilafrica/commit/b0399f0cef3a35375874c9fd63bdf6dda53166e9))

## [1.4.0](https://github.com/didi-rare/vigilafrica/compare/v1.3.7...v1.4.0) (2026-08-12)


### Features

* **api,web:** paginate the events list and make truncation visible ([#221](https://github.com/didi-rare/vigilafrica/issues/221)) ([343fc8b](https://github.com/didi-rare/vigilafrica/commit/343fc8bc63c03f729192075800eee3dc2895fb61))


### Bug Fixes

* **api:** correct open-events contract, test real middleware, buffer response ([087f236](https://github.com/didi-rare/vigilafrica/commit/087f236ecb5a82d52f3efa2e2a1c5536e6fbfc08))
* **api:** declare the 500 contract, harden the gate, repair records ([ad360b2](https://github.com/didi-rare/vigilafrica/commit/ad360b227d947c4a3a3e33df840d1e1ed2eeca00))
* **api:** reject non-finite coordinates, inject config, close review gaps ([3970985](https://github.com/didi-rare/vigilafrica/commit/3970985bd7f6f3af7b2f6ad31b201038eafe5130))
* **api:** repair invalid OpenAPI, add parse gate, stop silent fallbacks ([4d327a9](https://github.com/didi-rare/vigilafrica/commit/4d327a9ce0106d25333b7f838ed2fcc5f62545a5))
* **api:** unify client-IP resolution and add explicit lat/lng to /v1/context ([dc9897c](https://github.com/didi-rare/vigilafrica/commit/dc9897c87251f16889d857910a040a8084064265))
* **web:** bump nanoid to 3.3.18 to clear GHSA-2v37-7h3g-55p8 ([c264e38](https://github.com/didi-rare/vigilafrica/commit/c264e3818f7e55eb6d7cbc4818a0502780cf5c99))
* **web:** bump nanoid to 3.3.18 to clear GHSA-2v37-7h3g-55p8 ([bd4bc3d](https://github.com/didi-rare/vigilafrica/commit/bd4bc3d92ecd95943abfb8e6d5f1c1a19a39895d))

## [1.3.7](https://github.com/didi-rare/vigilafrica/compare/v1.3.6...v1.3.7) (2026-08-07)


### Bug Fixes

* **ci:** clear the brace-expansion advisory blocking every PR [trivial] ([#206](https://github.com/didi-rare/vigilafrica/issues/206)) ([e505526](https://github.com/didi-rare/vigilafrica/commit/e50552601e3242614d3a86e6f6b7e9e11fd8bd6c))
* **ci:** clear the js-yaml advisory blocking the promotion [trivial] ([#216](https://github.com/didi-rare/vigilafrica/issues/216)) ([cd0108f](https://github.com/didi-rare/vigilafrica/commit/cd0108fcdfc263cab032408c22b1378cd4d6af57))
* **ci:** clear the undici + fast-uri advisory batch [trivial] ([#207](https://github.com/didi-rare/vigilafrica/issues/207)) ([cb85df8](https://github.com/didi-rare/vigilafrica/commit/cb85df8a201eafcb12dc2d789a23ed4fbee995f7))


### Performance Improvements

* **api:** precompute admin boundary area for enrichment ([#211](https://github.com/didi-rare/vigilafrica/issues/211)) ([64ffd81](https://github.com/didi-rare/vigilafrica/commit/64ffd81906b3d0c6d2be41bdce6dd9d70af7a7c0))

## [1.3.6](https://github.com/didi-rare/vigilafrica/compare/v1.3.5...v1.3.6) (2026-07-29)


### Bug Fixes

* **web:** keep the dashboard loading text on screen inside the reservation ([#195](https://github.com/didi-rare/vigilafrica/issues/195)) ([24ae84a](https://github.com/didi-rare/vigilafrica/commit/24ae84a02d8aad19d4344e1580f2842731013a55))
* **web:** reserve viewport height for the lazy dashboard so visible content stops jumping ([#193](https://github.com/didi-rare/vigilafrica/issues/193)) ([30b2338](https://github.com/didi-rare/vigilafrica/commit/30b2338b93629648c58a560924be6a78b1cb3c3c))

## [1.3.5](https://github.com/didi-rare/vigilafrica/compare/v1.3.4...v1.3.5) (2026-07-26)


### Bug Fixes

* **ci:** clear the overnight web-audit advisory batch ([#183](https://github.com/didi-rare/vigilafrica/issues/183)) ([9c7fbe5](https://github.com/didi-rare/vigilafrica/commit/9c7fbe565c1bdb2245a83572e0a7f4f724702a71))
* **web:** reveal landing sections on load so they are never blank ([#182](https://github.com/didi-rare/vigilafrica/issues/182)) ([5b880c3](https://github.com/didi-rare/vigilafrica/commit/5b880c38874349814ab0492beeebb75a1c5d5930))

## [1.3.4](https://github.com/didi-rare/vigilafrica/compare/v1.3.3...v1.3.4) (2026-07-24)


### Bug Fixes

* **api:** log EventHandler 500s and add a panic recovery backstop ([#170](https://github.com/didi-rare/vigilafrica/issues/170)) ([cf1eab2](https://github.com/didi-rare/vigilafrica/commit/cf1eab205f68573404b0a5a5499faf5e1ae71576))
* **deps:** bump react-router-dom 7.17.0 -&gt; 7.18.1 to clear 4 advisories ([#177](https://github.com/didi-rare/vigilafrica/issues/177)) ([8671eaf](https://github.com/didi-rare/vigilafrica/commit/8671eaf080acf6903edeb79a21bb0f96f6a29aeb))

## [1.3.3](https://github.com/didi-rare/vigilafrica/compare/v1.3.2...v1.3.3) (2026-07-21)


### Bug Fixes

* **deps:** clear GO-2026-5970 (x/text) + GHSA-4c8g-83qw-93j6 (fast-uri) to unblock CI ([#161](https://github.com/didi-rare/vigilafrica/issues/161)) ([082be5f](https://github.com/didi-rare/vigilafrica/commit/082be5fc776a1304089e819ad558c539e848b455))

## [1.3.2](https://github.com/didi-rare/vigilafrica/compare/v1.3.1...v1.3.2) (2026-07-21)


### Bug Fixes

* **db:** purge events stored before the bbox containment guard existed ([7620eb3](https://github.com/didi-rare/vigilafrica/commit/7620eb39bcacdde7bc2482456c324d379812288d))
* **db:** purge events stored before the bbox containment guard existed ([0ac7361](https://github.com/didi-rare/vigilafrica/commit/0ac736165eecb8eec0f17907c0d7c0c5861100e1))
* **enrich:** label border-spillover events by country (ADM0 fallback) ([b81e783](https://github.com/didi-rare/vigilafrica/commit/b81e78367f701224a63f82e7fc804273372464f6))
* **enrich:** label border-spillover events by country (ADM0 fallback) ([9da4336](https://github.com/didi-rare/vigilafrica/commit/9da4336602b8c20f887386e18241097a34adc6b2))
* **ingestor:** address openspec-review findings on EONET closed-event fix ([f539a43](https://github.com/didi-rare/vigilafrica/commit/f539a43f67be8279d91a4265321d7ae6ecc13cf0))
* **ingestor:** query EONET for closed events so floods are ingested ([a661045](https://github.com/didi-rare/vigilafrica/commit/a6610454c05c650c771208097a02f5eac0c8585e))
* **ingestor:** query EONET for closed events so floods are ingested ([a9f599c](https://github.com/didi-rare/vigilafrica/commit/a9f599c206798c4039cddeea9adc203888ae5ddb))
* **ingest:** report unverified geometry per run, add border-case tests ([991a309](https://github.com/didi-rare/vigilafrica/commit/991a30970113ce2065b5a5d6d73a027cc773306a))
* **ingest:** validate event coordinates against the country bbox ([53e7e6b](https://github.com/didi-rare/vigilafrica/commit/53e7e6bf0951663249b2965ffecce236b70093e2))
* **ingest:** validate event coordinates against the country bbox ([93ba370](https://github.com/didi-rare/vigilafrica/commit/93ba370482e26ae238d82e5aec2703c4835d8529))
* **web:** add SPA rewrite and per-env robots.txt + sitemap.xml ([acc973f](https://github.com/didi-rare/vigilafrica/commit/acc973fee14ad590acce718f08d918a0bf537f3d))
* **web:** add SPA rewrite and per-env robots.txt + sitemap.xml ([51962f4](https://github.com/didi-rare/vigilafrica/commit/51962f43018e18370b22c8d1e71faa1d51225696))
* **web:** resolve brace-expansion npm audit advisory ([7c6d76c](https://github.com/didi-rare/vigilafrica/commit/7c6d76c355f80e053ca655b03e0744bc935f5e68))
* **web:** resolve brace-expansion npm audit advisory (GHSA-3jxr-9vmj-r5cp) ([7fe6458](https://github.com/didi-rare/vigilafrica/commit/7fe6458599d44081553f48c1b0bb726e6046eeda))

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
