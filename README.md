# go-sort

按文件内声明排序的 Go 命令行工具。顺序为 import、游离注释、main/init、const、var、type 与其接收者方法、其他函数；同组 ASCII 大写首字母名称优先（沿用既有规则），然后按忽略大小写的名称排序，大小写相同时按原名称排序。多 spec 声明保持整个组不拆分，按首个 spec 的名称排序。

```sh
go install github.com/atcharles/go-sort@latest
go-sort .                 # 递归排序当前目录，不处理测试文件
go-sort -tests .          # 同时处理 *_test.go
go-sort test .            # -tests 的位置别名
go-sort -r=false -tests . # 仅当前目录，包含测试文件
go-sort -w=false .        # 解析与排序检查，不写入文件
go-sort path/to/file.go   # 排序单个非测试 Go 文件
```

默认值：路径 `.`，`-r=true`，`-tests=false`，`-w=true`。写入默认开启，请先提交或备份待处理代码；`-w=false` 仅检查能否排序，不输出源码、不以“需要格式化”作为失败。

**行为变化：修复前递归模式会意外包含测试文件；修复后需显式 `-tests` 或 `test`。** `go-sort .` 不含测试文件，单独指定 `x_test.go` 也需 `-tests`。`test` 只在第一个位置参数时作为别名，`go-sort test` 等于 `go-sort -tests .`；若目标目录恰好叫 test，请写 `go-sort ./test`。

保留的旧位置形式：无参数、`.`、`./`、空路径均指当前目录；文件和目录路径均支持；多个位置参数沿用“最后一个是路径”的规则，其余忽略（首个 `test` 除外）。`./...` 现在正式作为当前目录别名（旧版会在路径校验时报不存在），递归与否仍由 `-r` 决定。若路径匹配可执行文件路径的后缀（如 `go-sort go-sort`），也保留旧版回退当前目录的行为。使用标准库 flag：选项必须写在位置参数前，`--` 可结束选项解析；`go-sort test -r=false .` 中的 `-r=false` 不会作为选项解析，应写 `go-sort -r=false test .`。

遍历跳过 `.git` 和 `vendor`；读取或遍历失败会报告错误。工具沿用既有声明与注释处理规则，不替代 gofmt/goimports，也不负责跨文件依赖分析。

## 开发

使用 Go 1.27.1（两个 go.mod 均声明 `go 1.27.1`）。Go 会将与 go 版本相同的冗余 toolchain 指令移除；保留它会导致命令要求 go mod tidy，因此采用 Go 的标准化模块格式。`make help` 列出命令；`make fmt` 依次运行本项目的 `go run . -tests .`、gofmt、goimports。`make ci` 按顺序执行临时副本格式检查（无差异且不修改工作区）、lint、style-check、vuln、`test -race`。

`tools/go.mod` 用 tool 指令独立锁定 golangci-lint v2、govulncheck、goimports、noelse，统一经 `go tool -modfile=tools/go.mod` 调用。noelse 采用 tool 方式而不是无版本 `go run`，以保证规则版本可重现，并隔离私有 kit 的依赖图；`github.com/glibtools/kit` 为私有模块，开发机/CI 需配置 `GOPRIVATE=github.com/glibtools/*` 和 Git 只读访问权限。安装与使用 go-sort 本身无需访问 kit。

`.golangci.yml` 复制自 kit 的共享配置，保留全部规则与 depguard 禁用清单，并加强 govet shadow `strict: true`、exported 文档注释及 switch default 检查。代码与测试禁止 else。测试使用标准库 testing + go-cmp；黄金样例来自 main bbf5a84（EOF 回归样例的期望输出以补齐结尾换行后的 main 输出为准），修改排序须先更新测试并解释兼容性影响。

`make deps-check` 查看升级，`make deps-upgrade` 升级两个模块并执行 ci。Go 1.27.1 与工具版本按本次官方模块元数据核对；go-cmp、goimports、govulncheck 使用 Go Authors 的 BSD 许可，golangci-lint 为 GPL-3.0，仅作为独立开发工具运行，不链接进发布二进制。工具依赖量较大，因此与主模块分离；主模块仅为测试引入 go-cmp。

旧 tools.sh 含删除 .git、重建历史与强推操作，已删除；其 gosort 功能由 `make fmt` 取代。本项目是命令行排序工具，没有数据库、迁移或服务集成测试，因此只提供适用的工具链目标。

工具间接依赖暂缓升级：glob v1.0.0 移除了 `glob.Glob`，exhaustive v0.14.0 与 predeclared v0.3.0 移除了 golangci-lint v2.14.0 使用的导出 flag 常量，实际编译失败。因此分别锁定 glob v0.2.3、exhaustive v0.13.0、predeclared v0.2.2，其他依赖已尝试升级至最新稳定版；待新版 golangci-lint 支持这些 API 后，在 `make deps-upgrade` 移除兼容版本固定并重新执行 ci。

源码按命令行、遍历、声明索引与输出职责拆分，开发时用 `go run .`；go generate 的安装命令已同步改为安装整个包。

同时修复无结尾换行的文件：旧版可能丢失 package 行，或因取到文件末尾后的 NUL 而格式化失败；新版保留 package 与末尾注释并正常写出。已有成功排序输入的声明顺序保持不变。
