# pcurl
simple pressure test using curl

# Features
- [x] Parse curl to golang request
- [x] Support curl to replace placeholders with content in csv, and match the headers of csv one by one
- [x] Supports printing request and return information at sample rate
- [ ] ineer web ui
- [ ] statistics
- [ ] DAG or Workflow
----

# requests 是一个简单的压测工具

- [x] 基于 curl 命令，知道 curl 就能压测
- [x] 支持 curl 当中占位符替换为 csv内容，占位符和csv的header匹配
- [x] 支持按照采样率打印请求和返回信息
- [ ] 内嵌的界面
- [ ] 压测结果的统计
- [ ] 工作流

# 使用方法
```bash
.
├── README.md
├── curl // 放置curl 命令模板
├── data.txt // csv 数据
├── main.go 
└── go.mod // 包
└── requests_mac // bin
└── requests_linux // bin
```

压测命令例如：
```bash
./requests_mac -r 1 -n 1 -c ./curl -d ./data.txt -s 5 p
```

含义：
- -r 1 QPS
- -n 1 共个线程, 每个请求越耗时，则需要越大的并发数
- -c ./curl 使用这个目录下的curl模板
- -d ./data.txt 使用这个文件中的数据，如果不带这个参数则是根据固定的curl命令压测
- -s 5 没5个请求打印一次
- p 压测

curl -L -X POST 'google.com?s={{keyword}}'
