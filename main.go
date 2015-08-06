package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type tagslice []*string

func (i *tagslice) String() string {
	return fmt.Sprintf("%s", *i)
}

func (i *tagslice) Set(value string) error {
	*i = append(*i, &value)
	return nil
}

var (
	tags           tagslice
	region         = flag.String("region", "", "AWS region to use")
	purgeAfterDays = flag.Int("k", 0, "Purge snapshot after this many days. Zero value means never purge")
)

func main() {
	var err error
	var volumes []*ec2.Volume

	flag.Var(&tags, "tags", "Select EBS volumes using these tag keys e.g. 'Daily-Backup'. Tag values should be == 'true'")
	flag.Parse()

	if len(tags) == 0 {
		log.Fatal("You must specify at least one tag")
	}

	config := aws.NewConfig()
	if *region != "" {
		config.Region = region
	}
	svc := ec2.New(config)

	volumes, err = GetVolumes(svc)
	if err != nil {
		log.Fatal(err)
	}

	for _, volume := range volumes {
		csi := ec2.CreateSnapshotInput{}
		csi.VolumeID = volume.VolumeID
		volname, _ := GetTagValue("Name", volume.Tags)
		var description string
		if volname == "" {
			description = fmt.Sprintf("ec2ab %s  - %s", volume.VolumeID, time.Now())
		} else {
			description = fmt.Sprintf("ec2ab %s (%s) - %s", volname, volume.VolumeID, time.Now())

		}
		csi.Description = &description

		cso, err := svc.CreateSnapshot(&csi)
		if err != nil {
			log.Fatal(err)
		}

		err = CreateSnapshotTags(svc, *cso.SnapshotID, volname, *volume.VolumeID)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("volume %s, %v", volume.VolumeID, volume)
	}
}

func CreateSnapshotTags(svc *ec2.EC2, resourceID, volumeName, volumeID string) error {

	var nKey, nVal string

	var tags []*ec2.Tag

	nKey = "Name"

	if volumeName == "" {
		nVal = fmt.Sprintf("ec2ab %s", volumeID)
	} else {
		nVal = fmt.Sprintf("ec2ab %s (%s)", volumeName, volumeID)
	}
	tags = append(tags, &ec2.Tag{Key: &nKey, Value: &nVal})

	if *purgeAfterDays > 0 {
		var paKey, paVal string
		var pKey, pVal string
		paKey = "PurgeAfter"
		paVal = time.Now().String()
		tags = append(tags, &ec2.Tag{Key: &paKey, Value: &paVal})

		pKey = "PurgeAllow"
		pVal = "true"
		tags = append(tags, &ec2.Tag{Key: &pKey, Value: &pVal})
	}

	cti := ec2.CreateTagsInput{Tags: tags}
	cti.Resources = append(cti.Resources, &resourceID)

	_, err := svc.CreateTags(&cti)
	if err != nil {
		return err
	}

	return nil
}

func GetVolumes(svc *ec2.EC2) ([]*ec2.Volume, error) {
	dvi := ec2.DescribeVolumesInput{}

	for _, tag := range tags {
		filter := &ec2.Filter{}
		filterName := "tag:" + *tag
		filter.Name = &filterName
		value := "true"
		filter.Values = []*string{&value}
		dvi.Filters = append(dvi.Filters, filter)
	}

	fmt.Printf("filters %+v", dvi.Filters)

	dvo, err := svc.DescribeVolumes(&dvi)
	if err != nil {
		return nil, fmt.Errorf("describeVolumes error, %s", err)
	}

	fmt.Printf("dvo %+v", dvo)

	return dvo.Volumes, nil
}

// GetTagValue returns the value for the first matched key
func GetTagValue(key string, tags []*ec2.Tag) (string, bool) {
	for _, tag := range tags {
		if *tag.Key == key {
			return *tag.Value, true
		}
	}
	return "", false
}
