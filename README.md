## 使用方法
在用户目录下新建文件`.oos`,添加如下内容
```
endpoint="your endpoint"
accessKey="your accessKey"
secretKey="your secretKey"
```

```
./ctyun-oos-upload
  -b string
        存储桶(必传)
  -c int
        并发上传数 (default 10)
  -d string
        上传整个目录
  -f string
        上传指定文件
  -k string
        指定上传后的文件名
  -prefix string
        上传后文件前缀
  -skip value
        忽略文件的前缀
  -u    是否上传
  -v    打印文件名
```