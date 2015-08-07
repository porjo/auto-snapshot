package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type tagslice []*string

func (t tagslice) String() string {
	var str string
	for i, key := range t {
		if i != 0 {
			str += ", "
		}
		str += *key
	}
	return str
}

func (t *tagslice) Set(value string) error {
	*t = append(*t, &value)
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
		fmt.Println("You must specify at least one tag")
		os.Exit(1)
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

	if len(volumes) == 0 {
		log.Printf("No volumes found matching tags: %s\n", tags)
	}

	for _, volume := range volumes {
		csi := ec2.CreateSnapshotInput{}
		csi.VolumeID = volume.VolumeID
		volname, _ := getTagValue("Name", volume.Tags)
		var description string
		if volname == "" {
			description = fmt.Sprintf("ec2ab %s", *volume.VolumeID)
		} else {
			description = fmt.Sprintf("ec2ab %s (%s)", volname, *volume.VolumeID)

		}
		csi.Description = &description

		cso, err := svc.CreateSnapshot(&csi)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Snapshotting volume, Name: %s, VolumeID: %s\n", volname, *volume.VolumeID)

		err = CreateSnapshotTags(svc, *cso.SnapshotID, volname, *volume.VolumeID)
		if err != nil {
			log.Fatal(err)
		}

	}
}

func CreateSnapshotTags(svc *ec2.EC2, resourceID, volumeName, volumeID string) error {

	var nKey, nVal string

	var tags []*ec2.Tag

	nKey = "Name"

	if volumeName == "" {
		nVal = fmt.Sprintf("ec2ab %s, %s", volumeID, time.Now().Format("2006-01-02"))
	} else {
		nVal = fmt.Sprintf("ec2ab %s, %s", volumeName, time.Now().Format("2006-01-02"))
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

	dvo, err := svc.DescribeVolumes(&dvi)
	if err != nil {
		return nil, fmt.Errorf("describeVolumes error, %s", err)
	}

	return dvo.Volumes, nil
}

// getTagValue returns the value for the first matched key
func getTagValue(key string, tags []*ec2.Tag) (string, bool) {
	for _, tag := range tags {
		if *tag.Key == key {
			return *tag.Value, true
		}
	}
	return "", false
}
