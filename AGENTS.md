# GO-SORT 工程规则

- 项目代号：GO-SORT；Go 规则见 std-go-backend skill。
- 通用能力优先成熟库与标准库，依赖固定数字版本并提交锁文件。
- 禁止 else（含测试）、变量遮蔽与 `_` 前缀局部变量；错误用 `%w` 包装。
- exported 标识符注释以标识符名开头，switch 必须有 default。
- 修改排序前先补测试，声明顺序、类型组与注释输出由 testdata 黄金样例固定。
- 默认不处理测试；`-tests` 与首个位置别名 `test` 等价，选项须在路径前。
- 开发工具仅放 tools/go.mod，不污染主模块依赖图；禁止 testify。
- `make fmt` 使用本项目 `go run . -tests .`，再 gofmt、goimports。
- 交付前执行 `make ci`；涉及排序的改动还须对真实项目临时副本做兼容性比较。
- 不执行重建 Git 历史或强推的辅助脚本；不在原项目上做行为比较。
