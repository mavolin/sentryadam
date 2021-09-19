<div align="center">
<h1>sentryadam</h1>

[![Go Reference](https://pkg.go.dev/badge/github.com/mavolin/sentryadam.svg)](https://pkg.go.dev/github.com/mavolin/sentryadam)
[![GitHub Workflow Status (branch)](https://img.shields.io/github/workflow/status/mavolin/sentryadam/Test/v1?label=tests)](https://github.com/mavolin/sentryadam/actions)
[![codecov](https://codecov.io/gh/mavolin/sentryadam/branch/v1/graph/badge.svg?token=3qRIAudu4r)](https://codecov.io/gh/mavolin/sentryadam)
[![Go Report Card](https://goreportcard.com/badge/github.com/mavolin/sentryadam)](https://goreportcard.com/report/github.com/mavolin/sentryadam)
[![License](https://img.shields.io/github/license/mavolin/sentryadam)](https://github.com/mavolin/sentryadam/blob/v1/LICENSE)
</div>

---

Sentryadam is an extension for [adam](https://github.com/mavolin/adam) that adds support for [sentry](https://sentry.io).

## Main Features

* Support for adam as well as the `state.State` directly
* Add `*sentry.Hub`s to the `plugin.Context`/`event.Base`
* Monitor performance of individual commands or handlers
