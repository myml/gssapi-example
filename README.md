# 这是一个使用 golang 对接 gssapi 和 spengo 的例子

提前在 kerberos 创建 HTTP/b.example.com@EXAMPLE.COM主体，并导出 keytab 到~/b.keytab

kerberos 可使用 docker 运行调试 镜像 `gcavalcante8808/krb5-server`

## spengo-example-server

golang 实现的 spengo 客户端

使用`go run ./cmd/spengo-example-server -k ~/b.keytab -l :8080`运行

使用`curl --negotiate -vv -u : http://b.example.com:8080`访问，需提前使用 kinit 登录

chrome 和 firfox 也支持 spengo，需要额外配置

`google-chrome-stable --auth-server-whitelist="*example.com"`

## gssapi-example-server

golang 实现的 gssapi 服务端

`go run ./cmd/gssapi-example-server -k ~/b.keytab -l :8080`

cgo 调用 gssapi，需安装 `apt install libkrb5-dev`

## gssapi-example-client

golang 实现的 gssapi 客户端

`go run ./cmd/gssapi-example-client -s localhost:8080 -n HTTP/b.example.com`

cgo 调用 gssapi，需安装 `apt install libkrb5-dev`

需提前使用 kinit 登录
