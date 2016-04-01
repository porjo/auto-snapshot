package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
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

const (
	PurgeAfterKey       = "PurgeAfterr"
	PurgeAllowKey       = "PurgeAlloww"
	PurgeAfterFormat    = time.RFC3339
	MinSnapshotInterval = 15 //seconds
	MaxSnapshotRetries  = 3
)

var (
	tags           tagslice
	region         = flag.String("region", "", "AWS region to use")
	tagPrefix      = flag.String("tagPrefix", "auto-snap", "String to prefix to tag name, description")
	purgeAfterDays = flag.Int("k", 0, "Purge snapshot after this many days. Zero value means never purge")
	purgeSnapshots = flag.Bool("p", true, "Enable purging of snapshots")
)

func main() {

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
	svc := ec2.New(session.New(), config)

	err := CreateSnapshots(svc)
	if err != nil {
		log.Fatal(err)
	}

	if *purgeSnapshots {
		err = PurgeSnapshots(svc)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func CreateSnapshots(svc *ec2.EC2) error {
	volumes, err := GetBackupVolumes(svc)
	if err != nil {
		return err
	}

	if len(volumes) == 0 {
		log.Printf("No volumes found matching tags: %s\n", tags)
	}

	for _, volume := range volumes {
		csi := ec2.CreateSnapshotInput{}
		csi.VolumeId = volume.VolumeId
		volname, _ := getTagValue("Name", volume.Tags)
		var description string
		if volname == "" {
			description = fmt.Sprintf("%s: %s", *tagPrefix, *volume.VolumeId)
		} else {
			description = fmt.Sprintf("%s: %s (%s)", *tagPrefix, volname, *volume.VolumeId)
		}
		csi.Description = &description

		var err error
		var snap *ec2.Snapshot
		retries := 0
		for {
			snap, err = svc.CreateSnapshot(&csi)
			if err != nil {
				if aerr, ok := err.(awserr.Error); ok {
					retries++
					if retries > MaxSnapshotRetries {
						return fmt.Errorf("Maximum snapshot retries reached for volume %s", *volume.VolumeId)
					}
					if aerr.Code() == "SnapshotCreationPerVolumeRateExceeded" {
						sleep := time.Duration(MinSnapshotInterval+math.Pow(float64(5), float64(retries))) * time.Second
						log.Printf("SnapshotCreationPerVolumeRateExceeded, sleeping for %f\n", sleep.Seconds())
						time.Sleep(sleep)
					}
				} else {
					return err
				}
			} else {
				break
			}
		}

		log.Printf("snapshotting volume, Name: %s, VolumeId: %s, Size: %d GiB\n", volname, *volume.VolumeId, *volume.Size)

		err = CreateSnapshotTags(svc, *snap.SnapshotId, volname, *volume.VolumeId)
		if err != nil {
			return err
		}

		if len(volumes) > 1 {
			time.Sleep(MinSnapshotInterval)
		}
	}

	return nil
}

func PurgeSnapshots(svc *ec2.EC2) error {

	dsi := ec2.DescribeSnapshotsInput{}

	filter := &ec2.Filter{}
	filterName := "tag:" + PurgeAllowKey
	filter.Name = &filterName
	value := "true"
	filter.Values = []*string{&value}
	dsi.Filters = append(dsi.Filters, filter)

	dso, err := svc.DescribeSnapshots(&dsi)
	if err != nil {
		return fmt.Errorf("describeSnapshots error, %s", err)
	}

	purgeCount := 0
	for _, snapshot := range dso.Snapshots {
		var paVal string
		var found bool

		if paVal, found = getTagValue(PurgeAfterKey, snapshot.Tags); !found {
			log.Printf("snapshot ID %s has tag '%s' but does not have a '%s' tag. Skipping purge...", *snapshot.SnapshotId, PurgeAllowKey, PurgeAfterKey)
			continue
		}

		pa, err := time.Parse(PurgeAfterFormat, paVal)
		if err != nil {
			return err
		}

		if pa.Before(time.Now()) {
			deli := ec2.DeleteSnapshotInput{}
			deli.SnapshotId = snapshot.SnapshotId

			_, err := svc.DeleteSnapshot(&deli)
			if err != nil {
				return fmt.Errorf("error purging Snapshot ID %s, err %s", *snapshot.SnapshotId, err)
			}
			log.Printf("snapshot ID '%s' purged, size %d GiB\n", *snapshot.SnapshotId, *snapshot.VolumeSize)
			purgeCount++
		}
	}

	log.Printf("%d snapshots purged\n", purgeCount)

	return nil
}

func CreateSnapshotTags(svc *ec2.EC2, resourceID, volumeName, volumeID string) error {

	var nKey, nVal string

	var tags []*ec2.Tag

	nKey = "Name"

	if volumeName == "" {
		nVal = fmt.Sprintf("%s: %s, %s", *tagPrefix, volumeID, time.Now().Format("2006-01-02"))
	} else {
		nVal = fmt.Sprintf("%s: %s, %s", *tagPrefix, volumeName, time.Now().Format("2006-01-02"))
	}
	tags = append(tags, &ec2.Tag{Key: &nKey, Value: &nVal})

	if *purgeAfterDays > 0 {
		var paKey, paVal string
		var pKey, pVal string
		paKey = PurgeAfterKey
		paVal = time.Now().Add(time.Duration(*purgeAfterDays*24) * time.Hour).Format(PurgeAfterFormat)
		tags = append(tags, &ec2.Tag{Key: &paKey, Value: &paVal})

		pKey = PurgeAllowKey
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

func GetBackupVolumes(svc *ec2.EC2) ([]*ec2.Volume, error) {
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
