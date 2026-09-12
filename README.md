# libm3u8

m3u8 播放列表解析与 TS 分片拼接。

## 库

```go
// 三选一
m := libm3u8.NewFromURL(ctx, func() string { return u }, headers) // nextURL 返回空串即结束，可用于 live 轮询
m := libm3u8.NewFromReader(ctx, os.Stdin, headers, nil)           // 读到 EOF 结束
m, err := libm3u8.NewFromFile(ctx, "playlist.m3u8", headers, nil)

// 拼接为单一流
io.Copy(os.Stdout, m.Stream(func(u string) (io.ReadCloser, error) {
	return util.GetBody(ctx, u, headers)
}))

// 或按序遍历分片
for ts := range m.List() {
	fmt.Println(ts.URL(), ts.Duration())
}
```

## 命令

子命令均支持 `-H` 指定请求头，可重复、位置任意：`-H "Referer: http://xxx"`

| 命令 | 说明 |
| --- | --- |
| `./main play <url> \| mpv -` | 边下边播 |
| `./main list <url>` | 列出全部分片地址 |
| `./main pack <url> [前缀]` | 打包为单文件 |
| `./main serve [-p 端口] [-h 地址]` | 用 HTTP 提供打包文件 |

省略 URL 或子命令时，从标准输入读播放列表、输出到标准输出：

```
cat playlist.txt | ./main | mpv -
./main pack < playlist.txt | mpv -
```

`pack` 的文件名为 `前缀 + Unix 时间戳`，开头 64KB 为 JSON 索引 `[[时长,[偏移,长度]],...]`，其后为 TS 数据：

```
./main pack http://xxx           # 1700000000
./main pack http://xxx myvideo   # myvideo1700000000
./main pack http://xxx myvideo -H "Referer: http://xxx"
```

`serve` 用 HTTP 提供打包文件，`live=1` 仅保留最后 10 个分片，`live=0` 附带 `#EXT-X-ENDLIST`：

```
./main serve -p 6060 -h 127.0.0.1
# 播放列表 http://127.0.0.1:6060/1700000000.m3u8?live=0
# 分片     http://127.0.0.1:6060/1700000000.ts?range=65536-65543
```

## 打包（库）

```go
p := packer.New(m, "out.ts")
p.Limit(256) // header 空间，单位 KB，取值 4-512
p.Receive(func(size int64, free int) error {
	// free 为剩余 header 空间，较小时应让 nextURL 返回空串以平滑结束
	return nil
})
```
