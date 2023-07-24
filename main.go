// main of samples

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	oossdk "ctyun-oos-upload/oos"

	"github.com/gosuri/uilive"
	"github.com/pelletier/go-toml"
	"github.com/urfave/cli/v2"
)

var (
	errFileNotExists = errors.New("文件不存在")
)

func main() {
	app := &cli.App{
		UseShortOptionHandling: true,
		Usage:                  "天翼云OOS文件上传工具",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "bucket",
				Usage:    "存储桶(必传)",
				Aliases:  []string{"b"},
				Required: true,
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Usage:   "verbose",
				Aliases: []string{"v"},
			},
		},
		Commands: []*cli.Command{
			uploadCmd(),
			deleteCmd(),
			listCmd(),
			downloadCmd(),
		},
	}
	if err := app.Run(os.Args); err != nil {
		HandleError(err)
	}
}

func uploadCmd() *cli.Command {
	return &cli.Command{
		Name:  "upload",
		Usage: "上传文件",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "file",
				Usage:   "指定文件上传",
				Aliases: []string{"f"},
			},
			&cli.StringFlag{
				Name:    "dir",
				Usage:   "指定上传目录",
				Aliases: []string{"d"},
			},
			&cli.BoolFlag{
				Name:    "multipart",
				Usage:   "断点续传",
				Aliases: []string{"m"},
			},
			&cli.StringFlag{
				Name:  "prefix",
				Usage: "上传后文件前缀",
			},
			&cli.StringFlag{
				Name:  "skip",
				Usage: "忽略指定前缀的本地文件",
			},
			&cli.IntFlag{
				Name:    "concurrent",
				Usage:   "并发上传数量",
				Value:   10,
				Aliases: []string{"c"},
			},
			&cli.StringFlag{
				Name:    "block",
				Usage:   "分片大小",
				Value:   "5m",
				Aliases: []string{"b"},
			},
			&cli.BoolFlag{
				Name:    "upload",
				Usage:   "是否上传",
				Aliases: []string{"u"},
			},
			&cli.StringFlag{
				Name:    "key",
				Usage:   "上传后文件名",
				Aliases: []string{"k"},
			},
		},
		Action: func(ctx *cli.Context) error {
			oos := NewOos(ctx)
			if ctx.String("file") != "" {
				if ctx.Bool("multipart") {
					oos.uploadMultipart(ctx.String("file"), ctx.String("key"), ctx.String("prefix"), parseSize(ctx.String("block")), ctx.Int("concurrent"))
				} else {
					oos.uploadFile(ctx.String("file"), ctx.String("key"), ctx.String("prefix"))
				}
			} else if ctx.String("dir") != "" {
				oos.uploadDir(ctx.String("dir"), ctx.StringSlice("skip"), ctx.Int("concurrent"), ctx.Bool("upload"))
			}
			return nil
		},
	}
}

func deleteCmd() *cli.Command {
	return &cli.Command{
		Name:  "delete",
		Usage: "删除文件",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "file",
				Usage:   "指定文件",
				Aliases: []string{"f"},
			},
			&cli.StringFlag{
				Name:    "dir",
				Usage:   "指定目录",
				Aliases: []string{"d"},
			},
			&cli.StringFlag{
				Name:  "prefix",
				Usage: "前缀",
			},
		},
		Action: func(ctx *cli.Context) error {
			oos := NewOos(ctx)
			if ctx.String("file") != "" {
				oos.deleteFile(ctx.String("file"))
			} else if ctx.String("dir") != "" {
				oos.deleteDir(ctx.String("dir"))
			} else if ctx.String("prefix") != "" {
				oos.deleteDir(ctx.String("prefix"))
			}
			return nil
		},
	}
}

func listCmd() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "查看文件列表",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "prefix",
				Usage:    "文件名前缀",
				Required: true,
			},
		},
		Action: func(ctx *cli.Context) error {
			oos := NewOos(ctx)
			oos.listFile(ctx.String("prefix"))
			return nil
		},
	}
}

func downloadCmd() *cli.Command {
	return &cli.Command{
		Name:  "download",
		Usage: "下载文件",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "file",
				Usage:    "下载文件名",
				Aliases:  []string{"f"},
				Required: true,
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "输出文件名",
			},
			&cli.StringFlag{
				Name:    "block",
				Aliases: []string{"b"},
				Usage:   "分片大小",
			},
			&cli.BoolFlag{
				Name:    "multipart",
				Usage:   "是否分片下载",
				Aliases: []string{"m"},
			},
			&cli.IntFlag{
				Name:    "concurrent",
				Usage:   "并发下载数",
				Value:   3,
				Aliases: []string{"c"},
			},
		},
		Action: func(ctx *cli.Context) error {
			oos := NewOos(ctx)
			if ctx.Bool("multipart") {
				return oos.downloadMultipart(ctx.String("file"), ctx.String("output"), parseSize(ctx.String("block")), ctx.Int("concurrent"))
			} else {
				return oos.download(ctx.String("file"), ctx.String("output"))
			}
		},
	}
}

type Oos struct {
	client  *oossdk.Client
	bucket  *oossdk.Object
	verbose bool
}

func NewOos(ctx *cli.Context) *Oos {
	client := NewClient()
	bucket, err := client.Bucket(ctx.String("bucket"))
	if err != nil {
		HandleError(err)
	}
	return &Oos{client: client, bucket: bucket, verbose: ctx.Bool("verbose")}
}

func (oos *Oos) uploadFile(filePath, key, prefix string) {
	fi, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		HandleError(err)
	}
	if key == "" {
		key = fi.Name()
	}
	if prefix != "" {
		key = prefix + key
	}
	err = oos.bucket.PutObjectFromFile(key, filePath)
	if err != nil {
		HandleError(err)
	} else if oos.verbose {
		fmt.Println(key)
	}
}

func (oos *Oos) uploadDir(dir string, skip []string, concurrent int, upload bool) {
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
				if ok := strings.HasPrefix(objectKey, v); ok && oos.verbose {
					fmt.Println("忽略", objectKey)
					return nil
				}
			}
		}
		if upload {
			ch <- struct{}{}
			wg.Add(1)
			go func(objKey, p string) {
				e := oos.bucket.PutObjectFromFile(objKey, p)
				if e == nil {
					atomic.AddInt32(&c, 1)
					if oos.verbose {
						fmt.Println("上传文件", objectKey)
					} else {
						fmt.Fprintf(w, "已上传%d个文件\n", c)
					}
				} else {
					for i := 0; i < 5; i++ {
						e = oos.bucket.PutObjectFromFile(objKey, p)
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

func (oos *Oos) uploadMultipart(file, key, prefix string, block int64, concurrent int) error {
	fi, err := os.Stat(file)
	if os.IsNotExist(err) {
		return cli.Exit(errFileNotExists, 1)
	}
	if key == "" {
		key = fi.Name()
	}
	if prefix != "" {
		key = prefix + key
	}
	var listener = &ProgressListener{
		name: "上传",
		w:    uilive.New(),
	}
	for i := 1; ; i++ {
		fmt.Println("准备上传", file)
		err = oos.bucket.UploadFileWithCp(prefix+key, file, block, oossdk.Routines(concurrent), oossdk.Progress(listener), oossdk.Checkpoint(true, ".ucp"))
		if err != nil {
			if listener.Start {
				fmt.Printf("%v, 重试%d..\n", err, i)
			} else {
				return cli.Exit(err, 1)
			}
		} else {
			if oos.verbose {
				fmt.Println(key)
			}
			break
		}
	}
	return nil
}

func (oos *Oos) deleteFile(file string) error {
	ok, err := oos.bucket.IsObjectExist(file)
	if err != nil {
		return cli.Exit(err, 1)
	}
	if !ok {
		return cli.Exit(errFileNotExists, 1)
	}
	err = oos.bucket.DeleteObject(file)
	if err != nil {
		return cli.Exit(err, 1)
	}
	return nil
}

func (oos *Oos) deleteDir(dir string) error {
	pre := oossdk.Prefix(dir)
	marker := oossdk.Marker("")
	var c int
	w := uilive.New()
	w.Start()
	defer w.Stop()
	for {
		lor, err := oos.bucket.ListObjects(oossdk.MaxKeys(100), marker, pre)
		if err != nil {
			return cli.Exit(err, 1)
		}
		pre = oossdk.Prefix(lor.Prefix)
		marker = oossdk.Marker(lor.NextMarker)
		var objects []string
		for _, object := range lor.Objects {
			objects = append(objects, object.Key)
		}
		c += len(objects)
		_, err = oos.bucket.DeleteObjects(objects)
		if err != nil {
			return cli.Exit(err, 1)
		}
		fmt.Fprintf(w, "删除%d个文件\n", c)
		if !lor.IsTruncated {
			break
		}
	}
	return nil
}

func (oos *Oos) listFile(prefix string) error {
	pre := oossdk.Prefix(prefix)
	marker := oossdk.Marker("")
	var c int
	for {
		lor, err := oos.bucket.ListObjects(oossdk.MaxKeys(100), marker, pre)
		if err != nil {
			return cli.Exit(err, 1)
		}
		pre = oossdk.Prefix(lor.Prefix)
		marker = oossdk.Marker(lor.NextMarker)
		for _, object := range lor.Objects {
			fmt.Println(object.Key)
			c++
		}
		if !lor.IsTruncated {
			fmt.Println("共", c, "个文件")
			break
		}
	}
	return nil
}

func (oos *Oos) download(file, output string) error {
	ok, err := oos.bucket.IsObjectExist(file)
	if err != nil {
		return cli.Exit(err, 1)
	}
	if !ok {
		return cli.Exit(errFileNotExists, 1)
	}

	if output == "" {
		_, output = filepath.Split(file)
	}
	err = oos.bucket.GetObjectToFile(file, output)
	if err != nil {
		return cli.Exit(err, 1)
	}
	return nil
}

func (oos *Oos) downloadMultipart(file, output string, block int64, concurrent int) error {
	ok, err := oos.bucket.IsObjectExist(file)
	if err != nil {
		return cli.Exit(err, 1)
	}
	if !ok {
		return cli.Exit(errFileNotExists, 1)
	}

	if output == "" {
		_, output = filepath.Split(file)
	}
	var listener = &ProgressListener{
		name: "下载",
		w:    uilive.New(),
	}

	for i := 0; ; i++ {
		fmt.Println("准备下载", file)
		err = oos.bucket.DownloadFileWithCp(file, output, block, oossdk.Routines(concurrent), oossdk.Progress(listener), oossdk.Checkpoint(true, ".dcp"))
		if err != nil {
			if listener.Start {
				fmt.Fprintf(listener.w, "%v, 重试%d..\n", err, i)
			} else {
				return cli.Exit(err, 1)
			}
		} else {
			break
		}
	}
	return nil
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

func NewClient() *oossdk.Client {
	home, _ := os.UserHomeDir()
	config, err := toml.LoadFile(home + "/.oos")
	if err != nil {
		HandleError(err)
	}
	endpoint, accessKey, secretKey := config.Get("endpoint").(string), config.Get("accessKey").(string), config.Get("secretKey").(string)
	if !strings.HasPrefix(endpoint, "http") {
		endpoint = "http://" + endpoint
	}
	timeOut := oossdk.Timeout(30, 90)
	clientOptionV4 := oossdk.V4Signature(true)
	isEnableSha256 := oossdk.EnableSha256ForPayload(false)
	client, err := oossdk.New(endpoint, accessKey, secretKey, clientOptionV4, isEnableSha256, timeOut)
	if err != nil {
		HandleError(err)
	}
	return client
}

type ProgressListener struct {
	Start bool
	name  string
	w     *uilive.Writer
}

func (l *ProgressListener) ProgressChanged(event *oossdk.ProgressEvent) {
	switch event.EventType {
	case oossdk.TransferStartedEvent:
		l.w.Start()
		l.Start = true
		fmt.Fprintf(l.w, "开始%s..\n", l.name)
	case oossdk.TransferDataEvent:
		fmt.Fprintf(l.w, "%s.. %.2f%%/%s\n", l.name, float64(event.ConsumedBytes*100)/float64(event.TotalBytes), humanFileSize(float64(event.TotalBytes)))
	case oossdk.TransferCompletedEvent:
		fmt.Fprintf(l.w, "%s完成\n", l.name)
		l.w.Stop()
	case oossdk.TransferFailedEvent:
		fmt.Printf("%s失败\n", l.name)
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

func parseSize(size string) int64 {
	var units = map[string]int64{
		"B":  1,
		"KB": 1024,
		"K":  1024,
		"MB": 1024 * 1024,
		"M":  1024 * 1024,
		"GB": 1024 * 1024 * 1024,
		"G":  1024 * 1024 * 1024,
		"TB": 1024 * 1024 * 1024 * 1024,
	}
	sizeRegexp := regexp.MustCompile(`(\d+(\.\d+)?)((K|M|G|T)?B?)?`)
	part := sizeRegexp.FindAllStringSubmatch(strings.ToUpper(size), -1)
	if part == nil {
		fmt.Println(size, "格式错误")
		os.Exit(1)
	}
	if len(part[0]) == 1 {
		s, _ := strconv.ParseFloat(part[0][1], 64)
		return int64(s)
	} else {
		s, _ := strconv.ParseFloat(part[0][1], 64)
		return int64(float64(units[part[0][3]]) * s)
	}
}
