## 使用方法
在用户目录下新建文件`.oos`,添加如下内容
```
endpoint="your endpoint"
accessKey="your accessKey"
secretKey="your secretKey"
```

```
NAME:
   ctyun-oos-upload - 天翼云OOS文件上传工具

USAGE:
   ctyun-oos-upload [global options] command [command options] [arguments...]

COMMANDS:
   upload    上传文件
   delete    删除文件
   list      查看文件列表
   download  下载文件
   help, h   Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --bucket value, -b value  存储桶(必传)
   --verbose, -v             verbose (default: false)
   --help, -h                show help
```

### 上传

```
NAME:
   ctyun-oos-upload upload - 上传文件

USAGE:
   ctyun-oos-upload upload [command options] [arguments...]

OPTIONS:
   --file value, -f value        指定文件上传
   --dir value, -d value         指定上传目录
   --multipart, -m               断点续传 (default: false)
   --prefix value                上传后文件前缀
   --skip value                  忽略指定前缀的本地文件
   --concurrent value, -c value  并发上传数量 (default: 0)
   --block value, -b value       分片大小
   --upload, -u                  是否上传 (default: false)
   --key value, -k value         上传后文件名
   --help, -h                    show help
```

### 下载
```
NAME:
   ctyun-oos-upload download - 下载文件

USAGE:
   ctyun-oos-upload download [command options] [arguments...]

OPTIONS:
   --file value, -f value        下载文件名
   --output value, -o value      输出文件名
   --block value, -b value       分片大小
   --multipart, -m               是否分片下载 (default: false)
   --concurrent value, -c value  并发下载数 (default: 0)
   --help, -h                    show help
```

### 查看文件列表
```
NAME:
   ctyun-oos-upload list - 查看文件列表

USAGE:
   ctyun-oos-upload list [command options] [arguments...]

OPTIONS:
   --prefix value  文件名前缀
   --help, -h      show help
```

### 删除文件
```
NAME:
   ctyun-oos-upload delete - 删除文件

USAGE:
   ctyun-oos-upload delete [command options] [arguments...]

OPTIONS:
   --file value, -f value  指定文件
   --dir value, -d value   指定目录
   --prefix value          前缀
   --help, -h              show help
```