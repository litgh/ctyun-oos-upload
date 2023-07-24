// main of samples

package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"oos-go-sdk/oos"

	"github.com/gosuri/uilive"
	"github.com/pelletier/go-toml"
)

type arrayFlags []string

func (i *arrayFlags) String() string {
	return strings.Join(*i, ",")
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var (
	bucket     string
	dir        string
	file       string
	key        string
	prefix     string
	upload     bool
	concurrent int
	verbose    bool
	multipart  bool
	skip       arrayFlags
	del        bool
	buf        int64
)

func main() {
	flag.StringVar(&bucket, "b", "", "存储桶(必传)")
	flag.StringVar(&dir, "d", "", "上传整个目录")
	flag.StringVar(&file, "f", "", "上传指定文件")
	flag.StringVar(&key, "k", "", "指定上传后的文件名")
	flag.StringVar(&prefix, "prefix", "", "上传后文件前缀")
	flag.BoolVar(&upload, "u", false, "是否上传")
	flag.BoolVar(&verbose, "v", false, "打印文件名")
	flag.IntVar(&concurrent, "c", 10, "并发上传数")
	flag.BoolVar(&multipart, "m", false, "分片上传")
	flag.BoolVar(&del, "delete", false, "删除文件")
	flag.Int64Var(&buf, "buf", 5*1024*1024, "分片大小, 默认5*1024*1024")
	flag.Var(&skip, "skip", "忽略文件的前缀")
	flag.Parse()

	if bucket == "" {
		flag.Usage()
		os.Exit(1)
	}
	if del {
		if prefix == "" {
			flag.Usage()
			os.Exit(1)
		}
	} else if dir == "" && file == "" {
		flag.Usage()
		os.Exit(1)
	}
	client := NewClient()
	bucket, err := client.Bucket(bucket)
	if err != nil {
		HandleError(err)
	}
	if del {
		delObjects(prefix, bucket)
		return
	}

	if dir != "" {
		walkDir(dir, bucket)
	} else if file != "" {
		if multipart {
			uploadMultipart(file, key, prefix, bucket)
		} else {
			putFile(file, key, prefix, bucket)
		}
	}

	// /*************** bucket test *******************/
	// sample.CreateBucketSample()
	// sample.GetBucketLocation()
	// sample.BucketACLSample()
	// sample.DeleteBucketSample()
	// sample.BucketPolicySample()
	// sample.BucketWebSiteSample()
	// sample.BucketLoggingSample()
	// sample.BucketLifecycleSample()
	// sample.BucketCorsSample()
	// sample.BucketObjectLockSample()

	// /*************** AccessKey test *******************/
	// sample.AccessKeySample() // 6版本 只支持 https类型的endpoint 只支持V4签名

	// /*************** object test ***************/
	// sample.PutObjectSample()
	// sample.DeleteObjectSample()
	// sample.ListObjectsSample()
	// sample.GetObjectSample()
	// sample.CopyObjectSample()
	// sample.SignURLSample()

	// /*************** object multipart test ***************/
	// sample.StepMultipartSample()
	// sample.PutObjectMultipartSample()
	// sample.GetObjectMultipartSample()
	// sample.CopyPartMultipartSample()

	// /*************** service test ***************/
	// sample.ListBucketsSample()
	// sample.GetRegionSample()

	// fmt.Println("All samples completed")
}

func NewClient() *oos.Client {
	home, _ := os.UserHomeDir()
	config, err := toml.LoadFile(home + "/.oos")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	endpoint, accessKey, secretKey := config.Get("endpoint").(string), config.Get("accessKey").(string), config.Get("secretKey").(string)
	if !strings.HasPrefix(endpoint, "http://") {
		endpoint = "http://" + endpoint
	}
	timeOut := oos.Timeout(30, 90)
	clientOptionV4 := oos.V4Signature(true)
	isEnableSha256 := oos.EnableSha256ForPayload(false)
	client, err := oos.New(endpoint, accessKey, secretKey, clientOptionV4, isEnableSha256, timeOut)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return client
}

func delObjects(prefix string, bucket *oos.Object) {
	pre := oos.Prefix(prefix)
	marker := oos.Marker("")
	var c int
	w := uilive.New()
	w.Start()
	defer w.Stop()
	for {
		lor, err := bucket.ListObjects(oos.MaxKeys(100), marker, pre)
		if err != nil {
			HandleError(err)
		}
		pre = oos.Prefix(lor.Prefix)
		marker = oos.Marker(lor.NextMarker)
		var objects []string
		for _, object := range lor.Objects {
			objects = append(objects, object.Key)
		}
		c += len(objects)
		_, err = bucket.DeleteObjects(objects)
		if err != nil {
			HandleError(err)
		}
		fmt.Fprintf(w, "删除%d个文件\n", c)
		if !lor.IsTruncated {
			break
		}
	}
}

func uploadMultipart(file, key, prefix string, bucket *oos.Object) {
	fi, err := os.Stat(file)
	if os.IsNotExist(err) {
		fmt.Println(dir, "文件不存在")
		os.Exit(1)
	}
	if key == "" {
		key = fi.Name()
	}
	if prefix != "" {
		key = prefix + key
	}
	var listener = &UplaodListener{
		w: uilive.New(),
	}

	err = bucket.UploadFile(prefix+key, file, 5*1024*1024, oos.Routines(concurrent), oos.Progress(listener), oos.Checkpoint(true, "checkpointFile.ucp"))
	if err != nil {
		fmt.Println(err)
	} else if verbose {
		fmt.Println(key)
	}
}

type UplaodListener struct {
	w *uilive.Writer
}

func (l *UplaodListener) ProgressChanged(event *oos.ProgressEvent) {
	switch event.EventType {
	case oos.TransferStartedEvent:
		l.w.Start()
	case oos.TransferDataEvent:
		fmt.Fprintf(l.w, "Upload.. %.2f%%/%s\n", float64(event.ConsumedBytes*100)/float64(event.TotalBytes), humanFileSize(float64(event.TotalBytes)))
	case oos.TransferCompletedEvent:
		fmt.Fprintln(l.w, "上传完成")
		l.w.Stop()
	case oos.TransferFailedEvent:
		fmt.Println("上传失败")
		l.w.Stop()
	}
}

func putFile(file, key, prefix string, bucket *oos.Object) {
	fi, err := os.Stat(file)
	if os.IsNotExist(err) {
		fmt.Println(dir, "文件不存在")
		os.Exit(1)
	}
	if key == "" {
		key = fi.Name()
	}
	if prefix != "" {
		key = prefix + key
	}
	err = bucket.PutObjectFromFile(key, file)
	if err != nil {
		fmt.Println(err)
	} else if verbose {
		fmt.Println(key)
	}
}

func walkDir(dir string, bucket *oos.Object) {
	dir = filepath.Clean(dir)
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		fmt.Println(dir, "目录不存在")
		os.Exit(1)
	}
	wg := sync.WaitGroup{}
	var c int32 = 0
	ch := make(chan struct{}, concurrent)
	defer close(ch)
	w := uilive.New()
	w.Start()
	defer w.Stop()
	var uploadFailed []string

	err = filepath.WalkDir(dir, func(fpath string, d fs.DirEntry, err error) error {
		if d.IsDir() || err != nil {
			return nil
		}
		objectKey := fpath
		if strings.HasPrefix(fpath, dir) {
			objectKey = fpath[len(dir)+1:]
		}
		if len(skip) > 0 {
			for _, v := range skip {
				if ok := strings.HasPrefix(objectKey, v); ok && verbose {
					fmt.Println("忽略", objectKey)
					return nil
				}
			}
		}
		if upload {
			ch <- struct{}{}
			wg.Add(1)
			go func(objKey, p string) {
				e := bucket.PutObjectFromFile(objKey, p)
				if e == nil {
					atomic.AddInt32(&c, 1)
					if verbose {
						fmt.Println("上传文件", objectKey)
					} else {
						fmt.Fprintf(w, "已上传%d个文件\n", c)
					}
				} else {
					for i := 0; i < 5; i++ {
						e = bucket.PutObjectFromFile(objKey, p)
						if e == nil {
							break
						}
					}
					if e != nil {
						uploadFailed = append(uploadFailed, p)
					}
				}
				<-ch
				wg.Done()
			}(objectKey, fpath)
		} else {
			fmt.Println(fpath)
		}
		return err
	})
	if err != nil {
		fmt.Println(err)
	}
	wg.Wait()
	if upload {
		fmt.Printf("上传完成, 共 %d 个", c)
		if len(uploadFailed) > 0 {
			fmt.Printf(", 失败%d \n", len(uploadFailed))
			for _, v := range uploadFailed {
				fmt.Println(v)
			}
		}
	}
}

func humanFileSize(bytes float64) string {
	var thresh float64 = 1024
	var size = []string{"B", "KB", "MB", "GB", "TB"}
	var order int
	for ; bytes >= thresh && order < len(size)-1; order++ {
		bytes = bytes / thresh
	}
	return fmt.Sprintf("%.2f%s", bytes, size[order])
}

func HandleError(err error) {
	fmt.Println("occurred error:", err)
	os.Exit(1)
}
