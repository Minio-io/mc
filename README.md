# Minio Client Quickstart Guide [![Slack](https://slack.minio.io/slack?type=svg)](https://slack.minio.io)

Minio Client (minioc) provides a modern alternative to UNIX commands like ls, cat, cp, mirror, diff etc. It supports filesystems and Amazon S3 compatible cloud storage service (AWS Signature v2 and v4).

```

ls            List files and folders.
mb            Make a bucket or folder.
cat           Display contents of a file.
pipe          Write contents of stdin to target. When no target is specified, it writes to stdout.
share         Generate URL for sharing.
cp            Copy one or more objects to a target.
mirror        Mirror folders recursively from a single source to single destination.
diff          Compute differences between two folders.
rm            Remove file or bucket [WARNING: Use with care].
events        Manage bucket notification.
watch         Watch for events on object storage and filesystem.
policy	      Set public policy on bucket or prefix.
session       Manage saved sessions of cp and mirror operations.
config        Manage configuration file.
update        Check for a new software update.
version       Print version.

```

## Docker Container
### Stable
```
docker pull minio/minioc
docker run minio/minioc ls play
```

### Edge
```
docker pull minio/minioc:edge
docker run minio/minioc ls play
```

## macOS
### Homebrew
Install minioc packages using [Homebrew](http://brew.sh/)

```sh
brew install minio-minioc
minioc --help

```

## GNU/Linux
### Binary Download
| Platform | Architecture | URL |
| ---------- | -------- |------|
|GNU/Linux|64-bit Intel|https://dl.minio.io/client/minioc/release/linux-amd64/minioc|
||32-bit Intel|https://dl.minio.io/client/minioc/release/linux-386/minioc|
||32-bit ARM|https://dl.minio.io/client/minioc/release/linux-arm/minioc|

```sh

chmod +x minioc
./minioc --help

```

## Microsoft Windows
### Binary Download
| Platform | Architecture | URL |
| ---------- | -------- |------|
|Microsoft Windows|64-bit|https://dl.minio.io/client/minioc/release/windows-amd64/minioc.exe|
||32-bit|https://dl.minio.io/client/minioc/release/windows-386/minioc.exe |

```sh

minioc.exe --help

```

## FreeBSD
### Binary Download
| Platform | Architecture | URL |
| ---------- | -------- |------|
|FreeBSD|64-bit|https://dl.minio.io/client/minioc/release/freebsd-amd64/minioc|

```sh

chmod 755 minioc
./minioc --help

```

## Solaris/Illumos
### From Source

```sh

go get -u github.com/minio/minioc
minioc --help

```

## Install from Source
Source installation is intended only for developers and advanced users. `minioc update` command does not support update notifications for source based installations. Please download official releases from https://minio.io/downloads/#minio-client.

If you do not have a working Golang environment, please follow [How to install Golang](https://docs.minio.io/docs/how-to-install-golang).

```sh

go get -u github.com/minio/minioc

```

## Add a Cloud Storage Service
If you are planning to use `minioc` only on POSIX compatible filesystems, you may skip this step and proceed to [everyday use](#everyday-use).

To add one or more Amazon S3 compatible hosts, please follow the instructions below. `minioc` stores all its configuration information in ``~/.minioc/config.json`` file.

```sh

minioc config host add <ALIAS> <YOUR-S3-ENDPOINT> <YOUR-ACCESS-KEY> <YOUR-SECRET-KEY> <API-SIGNATURE>

```

Alias is simply a short name to you cloud storage service. S3 end-point, access and secret keys are supplied by your cloud storage provider. API signature is an optional argument. By default, it is set to "S3v4".

### Example - Minio Cloud Storage
Minio server displays URL, access and secret keys.

```sh

minioc config host add minio http://192.168.1.51 BKIKJAA5BMMU2RHO6IBB V7f1CwQqAcwo80UEIJEjc5gVQUSSx5ohQ9GSrr12 S3v4

```

### Example - Amazon S3 Cloud Storage
Get your AccessKeyID and SecretAccessKey by following [AWS Credentials Guide](http://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSGettingStartedGuide/AWSCredentials.html).

```sh

minioc config host add s3 https://s3.amazonaws.com BKIKJAA5BMMU2RHO6IBB V7f1CwQqAcwo80UEIJEjc5gVQUSSx5ohQ9GSrr12 S3v4

```

### Example - Google Cloud Storage
Get your AccessKeyID and SecretAccessKey by following [Google Credentials Guide](https://cloud.google.com/storage/docs/migrating?hl=en#keys)

```sh

minioc config host add gcs  https://storage.googleapis.com BKIKJAA5BMMU2RHO6IBB V8f1CwQqAcwo80UEIJEjc5gVQUSSx5ohQ9GSrr12 S3v2

```

NOTE: Google Cloud Storage only supports Legacy Signature Version 2, so you have to pick - S3v2

## Test Your Setup
`minioc` is pre-configured with https://play.minio.io:9000, aliased as "play". It is a hosted Minio server for testing and development purpose.  To test Amazon S3, simply replace "play" with "s3" or the alias you used at the time of setup.

*Example:*

List all buckets from https://play.minio.io:9000

```sh

minioc ls play
[2016-03-22 19:47:48 PDT]     0B my-bucketname/
[2016-03-22 22:01:07 PDT]     0B mytestbucket/
[2016-03-22 20:04:39 PDT]     0B mybucketname/
[2016-01-28 17:23:11 PST]     0B newbucket/
[2016-03-20 09:08:36 PDT]     0B s3git-test/

```
<a name="everyday-use"></a>
## Everyday Use
You may add shell aliases to override your common Unix tools.

```sh

alias ls='minioc ls'
alias cp='minioc cp'
alias cat='minioc cat'
alias mkdir='minioc mb'
alias pipe='minioc pipe'

```

## Explore Further
- [Minio Client Complete Guide](https://docs.minio.io/docs/minio-client-complete-guide)
- [Minio Quickstart Guide](https://docs.minio.io/docs/minio-quickstart-guide)
- [The Minio documentation website](https://docs.minio.io)

## Contribute to Minio Project
Please follow Minio [Contributor's Guide](https://github.com/minio/minioc/blob/master/CONTRIBUTING.md)

