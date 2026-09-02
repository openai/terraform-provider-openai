# Changelog

## [1.1.0](https://github.com/openai/terraform-provider-openai/compare/v1.0.0...v1.1.0) (2026-09-02)


### Features

* add organization and project spend limits ([#64](https://github.com/openai/terraform-provider-openai/issues/64)) ([bc100b2](https://github.com/openai/terraform-provider-openai/commit/bc100b20b2bfb66a99f474b30017c2138af62cad))


### Bug Fixes

* attest Terraform provider release artifacts ([#81](https://github.com/openai/terraform-provider-openai/issues/81)) ([abbd24d](https://github.com/openai/terraform-provider-openai/commit/abbd24d3eaf4e0d47c466ff84cc1a0d86c90fe58))
* **ci:** build Go sources for CodeQL ([#75](https://github.com/openai/terraform-provider-openai/issues/75)) ([4abb3a2](https://github.com/openai/terraform-provider-openai/commit/4abb3a227380cb50881a4fd2d54378347def97dc))
* harden signed release dependency resolution ([#72](https://github.com/openai/terraform-provider-openai/issues/72)) ([5778a57](https://github.com/openai/terraform-provider-openai/commit/5778a578a8212d594e8aaed41705201a0911de91))
* regenerate provider security hardening ([#77](https://github.com/openai/terraform-provider-openai/issues/77)) ([3cb63a4](https://github.com/openai/terraform-provider-openai/commit/3cb63a4119501f1ceca4f7573427b25c8cf58923))
* verify release artifact integrity before signing ([#74](https://github.com/openai/terraform-provider-openai/issues/74)) ([82c84d3](https://github.com/openai/terraform-provider-openai/commit/82c84d39f43017838748854a876952a207a9ab3d))


### Performance Improvements

* cache bounded project user reads ([#55](https://github.com/openai/terraform-provider-openai/issues/55)) ([74b2673](https://github.com/openai/terraform-provider-openai/commit/74b26730e3b44fd6e1ab1571a71ca745ca38771e))

## [1.0.0](https://github.com/openai/terraform-provider-openai/compare/v0.7.0...v1.0.0) (2026-07-29)


### ⚠ BREAKING CHANGES

* remove deprecated aggregate project rate limits resource ([#53](https://github.com/openai/terraform-provider-openai/issues/53))

### Features

* remove deprecated aggregate project rate limits resource ([#53](https://github.com/openai/terraform-provider-openai/issues/53)) ([fcc946c](https://github.com/openai/terraform-provider-openai/commit/fcc946c6bf63725e2e36b669040f7c0a367c9d33))


### Performance Improvements

* improve provider request resilience and telemetry ([#51](https://github.com/openai/terraform-provider-openai/issues/51)) ([ef1b6e5](https://github.com/openai/terraform-provider-openai/commit/ef1b6e5571394633a58f37602eb7366162815e65))

## [0.7.0](https://github.com/openai/terraform-provider-openai/compare/v0.6.1...v0.7.0) (2026-07-27)


### Performance Improvements

* optimize provider plan requests ([#38](https://github.com/openai/terraform-provider-openai/issues/38)) ([b969b4b](https://github.com/openai/terraform-provider-openai/commit/b969b4b))

### Bug Fixes

* address provider review regressions ([#40](https://github.com/openai/terraform-provider-openai/issues/40)) ([dd96f47](https://github.com/openai/terraform-provider-openai/commit/dd96f47))
* retry timed-out provider reads ([#43](https://github.com/openai/terraform-provider-openai/issues/43)) ([472c15c](https://github.com/openai/terraform-provider-openai/commit/472c15c))

### Documentation

* add security policy ([#31](https://github.com/openai/terraform-provider-openai/issues/31)) ([8584bbe](https://github.com/openai/terraform-provider-openai/commit/8584bbe))
* clarify project service account lifecycle documentation ([#49](https://github.com/openai/terraform-provider-openai/issues/49)) ([3d25706](https://github.com/openai/terraform-provider-openai/commit/3d25706))
* document project roles and improve rate-limit handling ([#50](https://github.com/openai/terraform-provider-openai/issues/50)) ([be43cff](https://github.com/openai/terraform-provider-openai/commit/be43cff))
* regenerate field-specific documentation examples ([#29](https://github.com/openai/terraform-provider-openai/issues/29)) ([e943e14](https://github.com/openai/terraform-provider-openai/commit/e943e14))

### Dependencies and Security

* add advanced CodeQL workflow for complete Go coverage ([#33](https://github.com/openai/terraform-provider-openai/issues/33)) ([e1ddf60](https://github.com/openai/terraform-provider-openai/commit/e1ddf60))
* bump GitHub Actions dependencies ([#7](https://github.com/openai/terraform-provider-openai/issues/7)) ([c85f8e5](https://github.com/openai/terraform-provider-openai/commit/c85f8e5))
* bump `actions/setup-go` in the GitHub Actions group ([#42](https://github.com/openai/terraform-provider-openai/issues/42)) ([def215d](https://github.com/openai/terraform-provider-openai/commit/def215d))
* bump `github.com/openai/openai-go/v3` from 3.43.0 to 3.44.0 ([#41](https://github.com/openai/terraform-provider-openai/issues/41)) ([dd35d6e](https://github.com/openai/terraform-provider-openai/commit/dd35d6e))
* bump `github.com/openai/openai-go/v3` from 3.44.0 to 3.46.0 ([#46](https://github.com/openai/terraform-provider-openai/issues/46)) ([448169d](https://github.com/openai/terraform-provider-openai/commit/448169d))
* bump `golang.org/x/crypto` from 0.45.0 to 0.52.0 in `tools` ([#30](https://github.com/openai/terraform-provider-openai/issues/30)) ([a786d09](https://github.com/openai/terraform-provider-openai/commit/a786d09))
* bump `golang.org/x/crypto` from 0.50.0 to 0.52.0 ([#32](https://github.com/openai/terraform-provider-openai/issues/32)) ([1d146c1](https://github.com/openai/terraform-provider-openai/commit/1d146c1))
* bump `golang.org/x/net` from 0.54.0 to 0.55.0 ([#37](https://github.com/openai/terraform-provider-openai/issues/37)) ([6568bf5](https://github.com/openai/terraform-provider-openai/commit/6568bf5))
* bump `google.golang.org/grpc` from 1.79.3 to 1.82.1 ([#47](https://github.com/openai/terraform-provider-openai/issues/47)) ([d887888](https://github.com/openai/terraform-provider-openai/commit/d887888))

## [0.6.0](https://github.com/openai/terraform-provider-openai/compare/v0.5.0...v0.6.0) (2026-07-16)


### Features

* retain cache working set and deprecate aggregate rate limits ([#27](https://github.com/openai/terraform-provider-openai/issues/27)) ([f49becb](https://github.com/openai/terraform-provider-openai/commit/f49becbd52301b65bcb1907c18531f08d69add69))

## [0.5.0](https://github.com/openai/terraform-provider-openai/compare/v0.4.1...v0.5.0) (2026-07-15)


### Features

* add cached and aggregate project rate limits ([#25](https://github.com/openai/terraform-provider-openai/issues/25)) ([743a487](https://github.com/openai/terraform-provider-openai/commit/743a4871c45409038e405d2a241fd369d102ed43))

## [0.4.1](https://github.com/openai/terraform-provider-openai/compare/v0.4.0...v0.4.1) (2026-07-10)


### Bug Fixes

* preserve group role IDs ([#20](https://github.com/openai/terraform-provider-openai/issues/20)) ([fbad77a](https://github.com/openai/terraform-provider-openai/commit/fbad77a8c9806b61f64637301374fdd03a1f14cb))
* preserve project group role IDs ([#19](https://github.com/openai/terraform-provider-openai/issues/19)) ([2ffb770](https://github.com/openai/terraform-provider-openai/commit/2ffb7702831b69fd79e6fade3bb4fad37cb0bc14))

## [0.4.0](https://github.com/openai/terraform-provider-openai/compare/v0.3.1...v0.4.0) (2026-07-10)


### Features

* clarify API Platform provider setup ([#17](https://github.com/openai/terraform-provider-openai/issues/17)) ([50554da](https://github.com/openai/terraform-provider-openai/commit/50554da159566692e41591d4f8e9a211787fb4cf))

## [0.3.1](https://github.com/openai/terraform-provider-openai/compare/v0.3.0...v0.3.1) (2026-07-09)


### Bug Fixes

* link Admin API docs in README ([#14](https://github.com/openai/terraform-provider-openai/issues/14)) ([e3adde5](https://github.com/openai/terraform-provider-openai/commit/e3adde5df3b9cdb759b093180305e72933b7e4f0))

## [0.2.0](https://github.com/openai/terraform-provider-openai/compare/v0.1.0...v0.2.0) (2026-07-08)


### Features

* allow management of permissionless role ([4315b44](https://github.com/openai/terraform-provider-openai/commit/4315b447dea7d47847b86e6e0f8cba9f76cf63c2))
* make API base URL configurable ([44a90f4](https://github.com/openai/terraform-provider-openai/commit/44a90f4a8ba33700c8b8bee2a7b0aa187fd3eff2))

## 0.1.0 (2026-06-26)


### Bug Fixes

* create project service accounts without default role ([fbcdf0f](https://github.com/openai/terraform-provider-openai/commit/fbcdf0fe10067ec507663e25e696cd040a9f21e7))
* create project service accounts without default role ([351d574](https://github.com/openai/terraform-provider-openai/commit/351d574e128d19d5aab596cbfb29700534034bd6))
* Regenerate provider for project group refresh ([a41fc87](https://github.com/openai/terraform-provider-openai/commit/a41fc8707b70fdb2fe1b8fa5ba5a4d982968da5b))

## 0.1.0 (Unreleased)

Initial prelaunch provider scaffolding.
