package sample

import (
	"ctyun-oos-upload/oos"
	"fmt"
)

func GetBucketLocation() {
	// New client
	client := NewClient()
	ret, err := client.GetBucketLocation(bucketName)
	if err != nil {
		HandleError(err)
	}

	fmt.Println(ret.DataLocationType == oos.DataLocationTypeSpecified)
	fmt.Println(ret)
}
