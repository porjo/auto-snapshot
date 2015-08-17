# auto-snapshot

Automate EC2 volume snapshots. Inspired by [ec2-automate-backup](https://github.com/colinbjohnson/aws-missing-tools/tree/master/ec2-automate-backup) but with the following differences:

- doesn't depend on AWS cli tools
- timestamps stored in human readable form
- name and description tags include volume's friendly name, for easy identification

<img src="http://porjo.github.io/auto-snapshot/snapshots.png"></img>

## Usage

[Setup an EC2 instance with an appropriate IAM role to allow snapshots to be created/deleted.](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html)

```
$ ./auto-backup --help
Usage of ./auto-backup:
  -k=0: Purge snapshot after this many days. Zero value means never purge
  -p=true: Enable purging of snapshots
  -region="": AWS region to use
  -tagPrefix="auto-snap": String to prefix to tag name, description
  -tags=: Select EBS volumes using these tag keys e.g. 'Daily-Backup'. Tag values should be == 'true'
```

Typical usage:

```
./auto-snapshot -region="ap-southeast-2" -tags="Backup-Daily" -k=3
```

## Building

Get a [precompiled binary](https://github.com/porjo/auto-snapshot/releases) for Linux or compile from source using Go build tools

```
go get -u github.com/porjo/auto-snapshot
$GOPATH/bin/auto-snapshot
```
