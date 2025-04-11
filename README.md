## Manifestor

Manifestor is a command line tool for running plugins.  This tools can submit jobs to cloud and local compute providers but is primarily intended to support local development.  

Manifestor jobs are configured as a set of json files. At a minumum, you will need the following:

  - Compute File:  the compute file defines the job that ill be run by the manifestor.  It has the following attributes:
    - Name: the name of the job
    - Provider: the type of compute provider that will run the job
    - Plugins: the plugin manifests that are going to be run
    - Event: the list of compute manifest files that will be run in an event.

Refer to the 'data' folder for the hello-world example which runs a job using the local docker compute provider and executes a single event with two compute manifests that must be run in sequence.

``` bash
>./manifestor --envFile=.env-local-hw run data/hello-world/compute.json
```
An environment file is necessary to give the compute provider permissions to configure and copy/read payloads from a compute store.  A sample environemnt using a local instance of minio to emulate AWS S3 is:

``` bash
CC_AWS_ACCESS_KEY_ID=minioBucketIdKey
CC_AWS_SECRET_ACCESS_KEY=minioBucketSecretKey
CC_AWS_DEFAULT_REGION=us-east-1
CC_AWS_S3_BUCKET=ccstore
CC_AWS_ENDPOINT=http://localhost:9000
```

If you are running jobs that use Cloud Compute payloads, a minio bucket is neccesary to emulate the compute store.  To configure this bucket, run minio locally using the docker-compose.yml file in this repositories root directory:

``` bash
>docker compose up
```
Once minio is running, open your browser and login to minio on `http://localhost:9001` with the username and password from the docker-compose.yml file.

![image info](./docs/img/minio-login.png)

Select `Buckets` from the `Administrator` left panel menu and click the `Create Bucket` button.

Create a bucket similar to this:

![image info](./docs/img/create-bucket.png)

Select `Access Keys` from the `User` left panel menu and click the `Create access key` button.

Name this key however you like and this will be the ID/Secret key used by your compute environment file.

![image info](./docs/img/create-access-key.png)

For this example, and access key id of `OBFLI6AWPWRmaFxOn4Zw` was created with a corresponding secret of `B5Mjn5FtekP6wwBGNecnVejgyG0c9jiaGAjshNui`

Next select `Object Browser` from the `User` left panel menu, then select `compute-store` to view the objects in the store (which is currently empty).

![image info](./docs/img/object-browser.png)

Select the `Create new path` button to the right and create a path called `cc_store`.  This is a store used by Cloud Compute to manage payloads and other information that is transmitted to running jobs.

![image info](./docs/img/create-cc-store.png)

now create an environment file called `.env` with the keys and endpoints you just created:

``` bash
CC_AWS_ACCESS_KEY_ID=OBFLI6AWPWRmaFxOn4Zw
CC_AWS_SECRET_ACCESS_KEY=B5Mjn5FtekP6wwBGNecnVejgyG0c9jiaGAjshNui
CC_AWS_DEFAULT_REGION=us-east-1
CC_AWS_S3_BUCKET=compute-store
CC_AWS_ENDPOINT=http://localhost:9000
```

At this point you should be able to run the hello world example by executing the following:
``` bash
>./manifestor --envFile=.env run data/hello-world/compute.json
```







