# auto-snapshot

Automate EC2 volume snapshots. Inspired by [ec2-automate-backup](https://github.com/colinbjohnson/aws-missing-tools/tree/master/ec2-automate-backup) but with the following differences:

- doesn't depend on AWS cli tools
- timestamps stored in human readable form
- name and description tags include volume's friendly name, for easy identification

## Usage

Get a [precompiled binary](https://github.com/porjo/auto-snapshot/releases) or compile from source using Go build tools

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