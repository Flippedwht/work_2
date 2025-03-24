**文件说明**：
go版本为1.23.7 
全部带basic后缀为基础版leveldb文件
全部带ultra后缀为优化版leveldb文件

api为启动端口——8080为优化版端口，8081为基础版端口
分别运行server.go 与 basic——server.go即可

internal文件夹为设计文件夹包含

```
block		 区块设计
loader 		 json格式定义
metrics 	 监视器（已废弃）
skiplist 	 动态跳表设计
storage 	 大块存储设计
```
其余文件夹
```
reslut 		 为vegeta测试结果
targets		 为vegeta测试目标
test 		 文件夹为开发过程中测试文件，可忽略
tools		 为生成测试文件代码，可直接运行
.bat 		 文件为测试脚本
```
运行过程——

1、运行tools文件夹下的测试数据生成代码

2、开启对应端口

3、运行测试脚本或终端输入vegeta命令即可测试

```
生成测试目标
go run tools/targets_gen/targets_gen.go
生成测试数据：
go run tools/gen_bench_data.go
go run tools/gen_basic_bench_data/gen_basic_bench_data.go
启动服务：
go run cmd/server/main.go
go run cmd/server/basic/basic_main.go
测试cmd:
cd /d E:\VScode_project\work_2
vegeta attack -duration=30s -rate=1000 -targets=targets.txt | vegeta report
vegeta attack -duration=30s -rate=1000 -targets=targets_basic.txt | vegeta report
```

