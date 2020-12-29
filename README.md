# nerd

[![Go Report Card](https://goreportcard.com/badge/github.com/qvantel/nerd)](https://goreportcard.com/report/github.com/qvantel/nerd)
[![Build status](https://img.shields.io/docker/cloud/build/qvantel/nerd.svg)](https://hub.docker.com/r/qvantel/nerd/builds)
[![Docker pulls](https://img.shields.io/docker/pulls/qvantel/nerd.svg)](https://hub.docker.com/r/qvantel/nerd)
[![Release](https://img.shields.io/github/v/release/qvantel/nerd.svg)](https://github.com/qvantel/nerd/releases/latest)

Welcome to the nerd repo! This service offers machine learning capabilities through a simple API, thus allowing other
services to be smarter without requiring a huge effort.

## Index

- [Quick Start](#quick-start)
- [Requirements](#requirements)
- [Deployment](#deployment)
- [Use](#use)
    - [Collectors](#collectors)
        - [File](#file)
    - [Metrics Updates](#metrics-updates)
    - [Manual Training](#manual-training)
    - [Evaluating An Input](#evaluating-an-input)
    - [Listing Available Entities](#listing-available-entities)
    - [Health](#health)
- [Testing](#testing)

## Quick Start

If you just want to try nerd out and see what it can do, here is a quick guide for running a test setup with containers:

1. Start a Zookeeper instance (nerd currently requires [Kafka](https://kafka.apache.org/documentation/#gettingStarted)
which in turn requires [Zookeeper](https://zookeeper.apache.org/)):
    ```bash
    docker run -d --restart=unless-stopped \
      --log-driver json-file \
      -p 2181:2181 \
      --name zookeeper zookeeper:3.6.2
    ```
1. Start a Kafka broker:
    ```bash
    docker run -d --restart=unless-stopped \
      --log-driver json-file \
      -p 7203:7203 -p 7204:7204 -p 9092:9092 \
      -e "KAFKA_LISTENERS=PLAINTEXT://:9092" \
      -e "KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://host.docker.internal:9092" \
      -e "KAFKA_DEFAULT_REPLICATION_FACTOR=1" \
      -e "KAFKA_DELETE_TOPIC_ENABLE=true" \
      -e "KAFKA_ZOOKEEPER_CONNECT=host.docker.internal:2181" \
      -e "KAFKA_BROKER_ID=1" \
      -e "KAFKA_HEAP_OPTS=-Xmx4G -Xms4G" \
      -e "ZOOKEEPER_SESSION_TIMEOUT_MS=30000" \
      --name kafka wurstmeister/kafka:2.12-2.1.1
    ```
    If running on linux, you might need to include the following in the docker run command (anywhere between
    `docker run` and the image)(same for the next steps):
    ```bash
      --add-host host.docker.internal:host-gateway
    ```
1. Start a nerd instance:
    > Here we're going to run nerd using the filesystem to store the neural net's parameters and the series' points, if
    > you'd like a more performant setup, refer to the ["Requirements"](#requirements) section for instructions on how
    > to setup [Redis](https://redis.io/topics/introduction) and [Elasticsearch](https://www.elastic.co/elasticsearch/)
    > instead.
    ```bash
    docker run -d --restart=unless-stopped -m 64m \
      --log-opt max-size=5m --log-driver=json-file \
      -p 5400:5400 \
      -e "LOG_LEVEL=INFO" \
      -e "SD_KAFKA=host.docker.internal:9092" \
      --name nerd qvantel/nerd:0.3.0
    ```
1. Check that everything is up and running by going to http://localhost:5400 with your browser (if you see a welcome
message, everything is good)
    > Not seeing anything? You can check the nerd logs with `docker logs --tail 100 nerd` to see if there are any
    > errors
1. Train a network to detect forged banknotes:
    1. Download the dataset from the UCI ML repo [here](http://archive.ics.uci.edu/ml/machine-learning-databases/00267/data_banknote_authentication.txt)
        > Credit: Dua, D. and Graff, C. (2019). UCI Machine Learning Repository [http://archive.ics.uci.edu/ml].
        > Irvine, CA: University of California, School of Information and Computer Science.
    1. Shuffle the points:
        ```bash
        sort -R -o shuffled-dataset.txt data_banknote_authentication.txt
        ```
    1. Load the test data using the built-in file collector:
        > If you want, you can add `variance,skewness,curtosis,entropy,class` to the beginning of `shuffled-dataset.txt`
        > and use the `-headers` variable to properly label the values, otherwise names will be auto-generated
        ```bash
       docker run -it --rm \
         -v $PWD/shuffled-dataset.txt:/opt/docker/dataset \
         --entrypoint=/opt/docker/fcollect qvantel/nerd:0.3.0 \
           -batch 50 \
           -in 4 \
           -margin 0.4999999 \
           -producer "kafka" \
           -sep "," \
           -series "banknote-forgery-detection" \
           -targets "host.docker.internal:9092" \
           dataset
        ```
    1. Send a training request:
        > If you opted to add the headers in the previous step, use `["variance","skewness","curtosis","entropy"]` as
        > inputs and `["class"]` as the output instead of the values bellow
        ```bash
        curl -XPOST -H "Content-Type: application/json" --data @- \
            localhost:5400/api/v1/nets <<EOF
        {
            "errMargin": 0.4999999,
            "inputs": ["value-0", "value-1", "value-2", "value-3"],
            "outputs": ["value-4"],
            "required": 1372,
            "seriesID": "banknote-forgery-detection"
        }
        EOF
        ```
    1. Check out the resulting net by going to http://localhost:5400/api/v1/nets
1. Use the network:
    1. With an authentic note (the output should be closer to 0 than 1)
        ```bash
        NET=banknote-forgery-detection-8921e4a37dabacc06fec3318e908d9fe4eb75b46-7804b6fc74b5c0a74cc0820420fa0edf6b1a117c-mlp
        ENDPOINT=localhost:5400/api/v1/nets/$NET/evaluate

        curl -XPOST -H"Content-Type: application/json" --data @- \
            $ENDPOINT <<EOF
        {
            "value-0": 3.2403,
            "value-1": -3.7082,
            "value-2": 5.2804,
            "value-3": 0.41291
        }
        EOF
        ```
    1. With a forged note (the output should be closer to 1 than 0)
        ```bash
        curl -XPOST -H"Content-Type: application/json" --data @- \
            $ENDPOINT <<EOF
        {
            "value-0": -1.4377,
            "value-1": -1.432,
            "value-2": 2.1144,
            "value-3": 0.42067
        }
        EOF
        ```
    > Curious about how to programmatically get the id? It can be calculated like so:
    > `seriesID + "-" + sha1(inputs) + "-" + sha1(outputs) + "-" + netType` (try it out on go playground
    > [here](https://play.golang.org/p/fmLNLuJhvT5))

## Requirements

This service has the following dependencies:

### Kafka

Even though metrics updates can be sent through the rest API, it's better to use a service like Kafka (maybe nats in the
future) to decouple that interaction and benefit from built-in load balancing. When producing metrics updates, the
series ID should be used by the partitioning strategy so that we have a smaller chance of triggering training for the
same series twice.

For testing, the Zookeeper and Kafka docker run commands from the ["Quick Start"](#quick-start) section can be used.

### A Network Parameter Store

Currently, Redis (and the filesystem but that should only be used for testing).

When using Redis with Sentinel, the `ML_STORE_PARAMS` variable should be used (instead of `SD_REDIS`) like so:
```bash
  -e 'ML_STORE_PARAMS={"group": "<master>", "URLs": "<sen1-host>:<sen1-port>,...,<senN-host>:<senN-port>"}'
```
Where `group` contains the master group name and `URLs` the comma-separated list of Sentinel instance host:port pairs.

For testing, the following command can be used to start up a Redis replica:
```bash
docker run -d \
  --log-driver json-file \
  -p 6379:6379 \
  --name redis redis:6.0.9-alpine
```

### A Point Store

Currently, Elasticsearch (and the filesystem but that should only be used for testing).

If Elasticsearch is used:

- The `action.auto_create_index` setting must be set to `.watches,.triggered_watches,.watcher-history-*` otherwise it
will create non optimal mappings increasing the storage impact.

- Given how index refreshing works, the automatic training request for a series that gets a high number of metrics
updates in a very short period of time (less than a second)(possible when the lag is momentarily high for example) might
not get issued. To avoid this, it's recommended to include multiple points per update with a lower frequency rather than
sending one update per point as it is extracted.

For testing, it is possible to get a working Elasticsearch instance quickly with the following command:
```bash
docker run -d \
  --log-driver json-file \
  -p 9200:9200 -p 9300:9300 \
  -e "discovery.type=single-node" \
  -e "action.auto_create_index=.watches,.triggered_watches,.watcher-history-*" \
  --name elasticsearch elasticsearch:7.10.1
```

## Deployment

For a simple deployment, the following command can be used to start up a nerd instance that'll use Redis and
Elasticsearch (changing the ip:ports for those of the corresponding services in your setup):

```bash
docker run -d --restart=unless-stopped -m 64m \
  --log-opt max-size=5m --log-driver=json-file \
  -p 5400:5400 \
  -e "LOG_LEVEL=INFO" \
  -e "SD_ELASTICSEARCH=http://host.docker.internal:9200" \
  -e "SERIES_STORE_TYPE=elasticsearch" \
  -e "SD_KAFKA=host.docker.internal:9092" \
  -e "SD_REDIS=host.docker.internal:6379" \
  -e "ML_STORE_TYPE=redis" \
  --name nerd qvantel/nerd:0.3.0
```
> You can find all available tags [here](https://hub.docker.com/r/qvantel/nerd/tags)

The following environment variables are available:

| Variable                  | Required | Default                                | Description                                                                                                                                                                               |
|---------------------------|----------|----------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| LOG_LEVEL                 | NO       | INFO                                   | Application/root log level, supported values are `TRACE`, `DEBUG`, `INFO`, `WARNING` and `ERROR`                                                                                          |
| MARATHON_APP_DOCKER_IMAGE | NO       | qvantel/nerd:$VERSION?                 | Included in the `artifact_id` field of log messages, gets filled in automatically when ran through Marathon                                                                               |
| SERVICE_NAME              | NO       | nerd                                   | Included in the `service_name` field of the log messages                                                                                                                                  |
| SERVICE_5400_NAME         | NO       | $SERVICE_NAME                          | Included in the `service_name` field of the log messages. If set, overrides whatever is defined in `$SERVICE_NAME`                                                                        |
| ML_ACT_FUNC               | NO       | bipolar-sigmoid                        | Activation function for the neural nets. Currently, only `bipolar-sigmoid` is supported                                                                                                   |
| ML_ALPHA                  | NO       | 0.05                                   | Learning rate for the neural nets, a low number (<= 0.1) is recommended. In the future, nerd will automatically test a few values and choose the one with the highest accuracy            |
| ML_HLAYERS                | NO       | 1                                      | Number of hidden layers for MLP nets. Some series might benefit from more than one. In the future, nerd will automatically test a few values and choose the one with the highest accuracy |
| ML_NET                    | NO       | mlp                                    | Default net type to use. Currently, only multilayer perceptron nets are supported (`mlp`)                                                                                                 |
| ML_MAX_EPOCH              | NO       | 1000                                   | Maximum number of times the net should iterate over the training set if the tolerance is never met                                                                                        |
| ML_STORE_TYPE             | NO*      | file                                   | Storage adapter that should be used for keeping network parameters. Currently supported values are `file` (for testing) and `redis`                                                       |
| ML_STORE_PARAMS           | NO       | {"Path": "."}                          | Settings for the net params storage adapter                                                                                                                                               |
| SD_REDIS                  | NO       |                                        | Redis replica host:port. Serves as a shortcut for filling in `$ML_STORE_PARAMS` when selecting the `redis` adapter                                                                        |
| ML_TEST_SET               | NO       | 0.4                                    | Fraction of the patterns provided to the training function that should be put aside for testing the accuracy of the net after training (0.4 is usually a good value)                      |
| ML_TOLERANCE              | NO       | 0.1                                    | Mean squared error change rate at which the training should stop to avoid overfitting                                                                                                     |
| SERIES_FAIL_LIMIT         | NO       | 5                                      | Number of **subsequent** processing failures in the consumer service at which the instance should crash                                                                                   |
| SD_KAFKA                  | YES      |                                        | Comma separated list of Kafka broker host:port pairs                                                                                                                                      |
| SERIES_KAFKA_GROUP        | NO       | nerd                                   | Consumer group ID that the instance should use                                                                                                                                            |
| SERIES_KAFKA_TOPIC        | NO       | nerd-events                            | Topic from which metrics updates will be consumed                                                                                                                                         |
| SERIES_STORE_TYPE         | NO*      | file                                   | Storage adapter that should be used for storing time series. Currently supported values are `file` (for testing) and `elasticsearch`                                                      |
| SERIES_STORE_PARAMS       | NO       | {"Path": "."}                          | Settings for the time series storage adapter                                                                                                                                              |
| SERIES_STORE_PASS         | NO       | ""                                     | Password for the selected series store (if applicable)                                                                                                                                    |
| SERIES_STORE_USER         | NO       | ""                                     | User for the selected series store (if applicable)                                                                                                                                        |
| SD_ELASTICSEARCH          | NO       |                                        | Elasticsearch protocol://host:port. Serves as a shortcut for filling in `$SERIES_STORE_PARAMS` when selecting the `elasticsearch` adapter                                                 |
> \* While not strictly required for operation, the default value should be overridden for anything other than testing and even then, not all testing should be done with those values

## Use

Once the service has been deployed, it is possible to interact with it either through Kafka or the rest API.

### Collectors

These are lightweight components that can be used to import data from other services into nerd. To facilitate their
development, nerd exposes the `github.com/qvantel/nerd/api/types` and `github.com/qvantel/nerd/pkg/producer` modules
which include the types used by the rest and Kafka interfaces as well as ready-made methods for producing messages to
them.

#### File
At the time of writing, the only public collector is the one built into this project under the fcollect command, which
imports datasets from plain text files. It can be accessed from the container (as seen in the
["Quick Start"](#quick-start) section) by changing the entrypoint to `/opt/docker/fcollect` like so (anything placed
after the image will be passed to fcollect as an argument):
```bash
docker run -it --rm \
  -v $PWD/shuffled-dataset.txt:/opt/docker/dataset \
  --entrypoint=/opt/docker/fcollect \
  --name fcollect qvantel/nerd:0.3.0 -series "demo" -targets "http://host.docker.internal:5400" dataset
```
Where the `-series` and `-targets` flags as well as the path to the dataset (full or relative to `/opt/docker` inside
the container) are required. Additionally, the following flags can be used to change the behaviour of the tool:

| Flag      | Type     | Default       | Description                                                                                                                          |
|-----------|----------|---------------|--------------------------------------------------------------------------------------------------------------------------------------|
| -batch    | int      | 10            | Maximum number of points to bundle in a single metrics update                                                                        |
| -headers  | bool     | false         | If true, the first line will be used to name the values                                                                              |
| -in       | int      | 1             | Number of inputs, counted left to right, all others will be considered outputs                                                       |
| -margin   | float    | 0             | Maximum difference between a prediction and the expected value for it to still be considered correct                                 |
| -producer | string   | "rest"        | What producer to use. Supported values are `rest` and `kafka`                                                                        |
| -sep      | string   | " "           | String sequence that denotes the end of one field and the start of the next                                                          |
| -series   | string   | N/A           | ID of the series that these points belong to                                                                                         |
| -stage    | string   | "test"        | Category of the data, `production` for real world patterns, `test` for anything else                                                 |
| -targets  | string   | N/A           | Comma separated list of `protocol://host:port` for nerd instances when using `rest`, `host:port` of Kafka brokers when using `kafka` |
| -timeout  | duration | 15s           | Maximum time to wait for the production of a message                                                                                 |
| -topic    | string   | "nerd-events" | Where to produce the messages when using `kafka`                                                                                     |

### Metrics Updates

Metrics updates can be ingested through either the `$SERIES_KAFKA_TOPIC` topic in Kafka or the `/api/v1/series/process`
endpoint. In both cases the message must conform to the
[Cloud Events v1 specification](https://github.com/cloudevents/spec) where the metadata fields should be filled in as
follows:

| Field           | Value                                                                               |
|-----------------|-------------------------------------------------------------------------------------|
| datacontenttype | "application/json"                                                                  |
| dataschema      | "github.com/qvantel/nerd/api/types/"                                                |
| id              | (a unique string identifier for this event)                                         |
| source          | (name of the service that generated the event)                                      |
| specversion     | (cloud events spec version, should be "1.0")                                        |
| subject         | (the entity that we are reporting about, it can be an environment name for example) |
| type            | "com.qvantel.nerd.metricsupdate"                                                    |

Additionally, the data fields should be filled in like so:

| Field          | Considerations                                                                                                                                                                                                                                          |
|----------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| data.seriesID  | Should conform to `[a-z][a-z0-9\._\-]+` and reference what that data can be used to predict. For example, if it's generic enough to predict storage impact in any env that uses that product stack, it should contain the stack version but not the env |
| data.errMargin | Maximum difference between the expected and produced result to still be considered correct during testing. Currently, this margin will be applied to all outputs of networks generated automatically                                                    |
| data.labels    | Should include any labels that might be useful for filtering later. Note that `subject` and `data.stage` will be copied here automatically                                                                                                              |
| data.points    | All points for the same series ID must contain the same attributes (doesn't matter if they are noted as inputs or outputs although within the same metrics update they do have to all be categorized in the same way)                                   |
| data.stage     | Must be either `production` for production grade data (usually that which originates from real world usage) or `test` (for anything else). The message will not be processed if this field doesn't have a valid value                                   |

Example:

```json
{
    "data": {
        "seriesID": "heart-of-gold-lightbulb-usage",
        "errMargin": 0.1,
        "labels": {
            "captain": "Zaphod Beeblebrox"
        },
        "points": [
            {
                "inputs": {
                    "humans": 2,
                    "robots": 1,
                    "aliens": 2
                },
                "outputs": {
                    "lightbulbs-on": 1500
                },
                "timestamp": 777808800
            }
        ],
        "stage": "test"
    },
    "datacontenttype": "application/json",
    "dataschema": "github.com/qvantel/nerd/api/types/",
    "id": "1",
    "source": "test-script",
    "specversion": "1.0",
    "subject": "heart-of-gold",
    "type": "com.qvantel.nerd.metricsupdate"
}
```

### Manual Training

Even though the service will automatically schedule training when it has enough points of a series, it is still
possible to manually trigger training from any preexisting series. To do this, just post a training request to the
`/api/v1/nets` endpoint like so (where `$URL` contains the address of the nerd service):

```bash
curl -XPOST -H"Content-Type: application/json" --data @- \
    $URL/api/v1/nets <<EOF
{
    "errMargin": 0.4999999,
    "inputs": ["value-0", "value-1", "value-2", "value-3", "value-4", "value-5", "value-6", "value-7", "value-8"],
    "outputs": ["value-9", "value-10"],
    "required": 699,
    "seriesID": "testloadtestset"
}
EOF
```

Where, the fields contain the following information:

| Field     | Description                                                                                               |
|-----------|-----------------------------------------------------------------------------------------------------------|
| errMargin | Maximum difference between the expected and produced result to still be considered correct during testing |
| inputs    | Which of the series values should be used as inputs                                                       |
| outputs   | Which of the series values should be used as outputs                                                      |
| required  | Number of points from the series that should be used to train and test                                    |
| seriesID  | ID of the series that should be used for training                                                         |

### Evaluating An Input

Once a net has been trained, it can be exploited through the `/api/v1/nets/{id}/evaluate` endpoint like so (where `$URL`
contains the address of the nerd service and `$ID` the ID of the network):

> NOTE: The network ID is different from that of the series it comes from, as multiple nets could be (and are) created
> from a single series. You can obtain one from another though, like this: seriesID-sha1(inputs)-sha1(outputs)-netType
> where inputs is the alphabetically ordered concatenation of the inputs and the same goes for outputs (when it's an
> automatically created net, you will get one per output, so typically you will just need to hash one output).

```bash
curl -XPOST -H"Content-Type: application/json" --data @- \
    $URL/api/v1/nets/$ID/evaluate <<EOF
{
    "value-0": 5000,
    "value-1": 0.07,
    "value-2": 0.1,
    "value-3": 100,
    "value-4": 1,
    "value-5": 0.1,
    "value-6": 0.1,
    "value-7": 0.1,
    "value-8": 1000
}
EOF
```

Sample response:

```json
{"value-9":0.16796547}
```

### Listing Available Entities

- Nets:
  - Endpoint: `/api/v1/nets`
  - Method: GET
  - Params:
    - offset: Offset to fetch, 0 by default
    - limit: How many networks to fetch, the service might return more in some cases, 10 by default, 50 maximum
  - Returns: A `types.PagedRes` object and a 200 if successful, a `types.SimpleRes` object and a 400 or 500 if not (depending on the error)
  - Sample response:
```json
{
    "last": true,
    "next": 0,
    "results": [
        {
            "accuracy": 0.953405,
            "averages": {
                "value-0": 4461.905,
                "value-1": 0.032476295,
                "value-10": 0.35714287,
                "value-2": 0.032452486,
                "value-3": 29.095238,
                "value-4": 0.6552371,
                "value-5": 0.036119178,
                "value-6": 0.034857206,
                "value-7": 0.028881064,
                "value-8": 1664.2858,
                "value-9": 0.64285713
            },
            "deviations": {
                "value-0": 2755.941,
                "value-1": 0.03142687,
                "value-10": 0.47972888,
                "value-2": 0.030137872,
                "value-3": 29.314167,
                "value-4": 0.45890477,
                "value-5": 0.036751248,
                "value-6": 0.02476096,
                "value-7": 0.031203939,
                "value-8": 1788.0956,
                "value-9": 0.47972888
            },
            "errMargin": 0.5,
            "id": "testloadtestset-4821586b0eb056ef9bc6913b1e19b800a4b6c563-6b25e7592c075a7c638b2aabda1cc1c71442f4cb-mlp",
            "inputs": [
                "value-0",
                "value-1",
                "value-2",
                "value-3",
                "value-4",
                "value-5",
                "value-6",
                "value-7",
                "value-8"
            ],
            "outputs": [
                "value-9"
            ],
            "type": "mlp"
        },
        {
            "accuracy": 0.94623655,
            "averages": {
                "value-0": 4390.476,
                "value-1": 0.031095339,
                "value-10": 0.34761906,
                "value-2": 0.031690583,
                "value-3": 28.428572,
                "value-4": 0.6271415,
                "value-5": 0.035738226,
                "value-6": 0.03452388,
                "value-7": 0.028071541,
                "value-8": 1604.762,
                "value-9": 0.65238094
            },
            "deviations": {
                "value-0": 2791.8901,
                "value-1": 0.030394992,
                "value-10": 0.4767823,
                "value-2": 0.029760389,
                "value-3": 29.065111,
                "value-4": 0.43468502,
                "value-5": 0.03649372,
                "value-6": 0.02479531,
                "value-7": 0.030412318,
                "value-8": 1741.2527,
                "value-9": 0.4767823
            },
            "errMargin": 0.5,
            "id": "testloadtestset-4821586b0eb056ef9bc6913b1e19b800a4b6c563-025d516f7acf54b430a00be752f9e4c4e2d1eee7-mlp",
            "inputs": [
                "value-0",
                "value-1",
                "value-2",
                "value-3",
                "value-4",
                "value-5",
                "value-6",
                "value-7",
                "value-8"
            ],
            "outputs": [
                "value-10"
            ],
            "type": "mlp"
        }
    ]
}
```

- Series:
  - Endpoint: `/api/v1/series`
  - Method: GET
  - Returns: An array of `types.BriefSeries` objects and a 200 if successful, a `types.SimpleRes` object and a 500 if not
  - Sample response:
```json
[
    {
        "name": "testloadtestset",
        "count": 699
    }
]
```

- Points:
  - Endpoint: `/api/v1/series/{id}/points`
  - Method: GET
  - Params:
    - limit: How many points to fetch, 10 by default, 500 maximum
  - Returns: An array of `pointstores.Point` objects and a 200 if successful, a `types.SimpleRes` object and a 404 or 500 if not (depending on the error)
  - Sample response:
```json
[
    {
        "@timestamp": 777809498,
        "value-0": 1000,
        "value-1": 0.01,
        "value-10": 0,
        "value-2": 0.01,
        "value-3": 10,
        "value-4": 0.4,
        "value-5": 0.01,
        "value-6": 0.02,
        "value-7": 0.01,
        "value-8": 1000,
        "value-9": 1
    },
    {
        "@timestamp": 777809497,
        "value-0": 5000,
        "value-1": 0.07,
        "value-10": 1,
        "value-2": 0.1,
        "value-3": 100,
        "value-4": 1,
        "value-5": 0.1,
        "value-6": 0.1,
        "value-7": 0.1,
        "value-8": 1000,
        "value-9": 0
    }
]
```

### Health

- Startup probe: `/api/v1/health/startup`

## Testing
Run unit tests locally:

> NOTE: If an Elasticsearch instance is running at http://localhost:9200 when the tests are run, they will load a
> test series into it

- Using Go (fastest):
```bash
go test -cover ./...
```

- Using containers (slower but doesn't require to have go installed)
```bash
./unit-test.sh
```
