# go-sort

按文件内声明排序的 Go 命令行工具。顺序为 import、游离注释、main/init、const、var、type 与其接收者方法、其他函数；同组导出名称优先，然后按忽略大小写的名称排序，大小写相同时按原名称排序。多 spec 声明保持整个组不拆分，按首个 spec 的名称排序。

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

保留的旧位置形式：无参数、`.`、`./`、空路径与 `./...` 均指当前目录；文件和目录路径均支持；多个位置参数沿用“最后一个是路径”的规则，其余忽略（首个 `test` 除外）。`./...` 是当前目录的别名，递归与否仍由 `-r` 决定。使用标准库 flag：选项必须写在位置参数前，`--` 可结束选项解析；`go-sort test -r=false .` 中的 `-r=false` 不会作为选项解析，应写 `go-sort -r=false test .`。

遍历跳过 `.git` 和 `vendor`；读取或遍历失败会报告错误。工具沿用既有声明与注释处理规则，不替代 gofmt/goimports，也不负责跨文件依赖分析。
