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
	skip       arrayFlags
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
	flag.Var(&skip, "skip", "忽略文件的前缀")
	flag.Parse()

	if bucket == "" {
		flag.Usage()
		os.Exit(1)
	}

	if dir == "" && file == "" {
		flag.Usage()
		os.Exit(1)
	}
	client := NewClient()
	bucket, err := client.Bucket(bucket)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if dir != "" {
		walkDir(dir, bucket)
	} else if file != "" {
		putFile(file, key, prefix, bucket)
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
	err = bucket.PutObjectFromFile(prefix+key, file)
	if err != nil {
		fmt.Println(err)
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
	err = filepath.WalkDir(dir, func(fpath string, d fs.DirEntry, err error) error {
		if d.IsDir() || err != nil {
			return nil
		}
		objectKey := fpath
		if strings.HasPrefix(fpath, dir) {
			objectKey = strings.Replace(fpath, dir, "", -1)[1:]
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
				<-ch
				wg.Done()
				if e == nil {
					atomic.AddInt32(&c, 1)
					if verbose {
						fmt.Println("上传文件", objectKey)
					}
				}
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
	}
}
