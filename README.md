# nerd

[![Go Report Card](https://goreportcard.com/badge/github.com/qvantel/nerd)](https://goreportcard.com/report/github.com/qvantel/nerd)
[![Build status](https://img.shields.io/docker/cloud/build/qvantel/nerd.svg)](https://hub.docker.com/r/qvantel/nerd/builds)
[![Docker pulls](https://img.shields.io/docker/pulls/qvantel/nerd.svg)](https://hub.docker.com/r/qvantel/nerd)

Welcome to the nerd repo! This service offers machine learning capabilities through a simple API, thus allowing other
services to be smarter without requiring a huge effort.

## Requirements

This service has the following dependencies:

### Kafka

Even though metrics updates can be sent through the rest API, it's better use a service like Kafka (maybe nats in the
future) to decouple that interaction and benefit from built in load balancing. When producing metrics updates, the
series ID should be used by the partitioning strategy so that we have a smaller chance of triggering training for the
same series twice.

### A Network Parameter Store

Currently, Redis (and the filesystem but that should only be used for testing).

When using Sentinel with Redis, the `ML_STORE_PARAMS` variable should be used (instead of `SD_REDIS`) like so:
```bash
  -e 'ML_STORE_PARAMS={"group": "<master>", "URLs": "<sen1-host>:<sen1-port>,...,<senN-host>:<senN-port>"}'
```
Where `group` contains the master group name and `URLs` the comma-separated list of Sentinel instance host:port pairs.

### A Point Store

Currently, Elasticsearch (and the filesystem but that should only be used for testing). If Elasticsearch is used:

- The `action.auto_create_index` setting must be set to `.watches,.triggered_watches,.watcher-history-*` otherwise it
will create non optimal mappings increasing the storage impact.

- Given how index refreshing works, the automatic training request for a series that gets a high number of metrics
updates in a very short period of time (less than a second)(possible when the lag is momentarily high for example) might
not get issued. To avoid this, it's recommended to include multiple points per update with a lower frequency rather than
sending one update per point as it is extracted.

For testing, it is possible to get a working Elasticsearch instance quickly with the following command:
```bash
docker run -d \
  -p 9200:9200 -p 9300:9300 \
  -e "discovery.type=single-node" \
  -e "action.auto_create_index=.watches,.triggered_watches,.watcher-history-*" \
  --name elasticsearch elasticsearch:7.9.3
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
  -e "SD_KAFKA=192.168.56.11:9092" \
  -e "SD_REDIS=192.168.56.11:6379" \
  -e "ML_STORE_TYPE=redis" \
  --name nerd qvantel/nerd:0.2.1
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

### Metrics updates

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
| data.points    | All points for the same series ID must contain the same values (doesn't matter if they are noted as inputs or outputs although within the same metrics update they do have to all be categorized in the same way)                                       |
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

Even through the service will automatically schedule training when it has enough points of a series, it is still
possible to manually trigger training from any preexisting series. To do this, just post a training request to the
`/api/v1/nets` endpoint like so (where `$URL` contains the address of the nerd service):

```bash
curl -XPOST -H"Content-Type: application/json" --data @- \
    $URL/api/v1/nets <<EOF
{
    "errMargin": 0.49999999,
    "inputs": ["value-0", "value-1", "value-2", "value-8", "value-4", "value-5", "value-6", "value-7", "value-3"],
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
| inputs    | Which of the series values should be treated as inputs                                                    |
| outputs   | Which of the series values should be treated as outputs                                                   |
| required  | Number of points from the series that should be used to train and test                                    |
| seriesID  | ID of the series that should be used for training                                                         |

### Evaluating an input

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

### Listing available entities

- Nets:
  - Endpoint: `/api/v1/nets`
  - Method: GET
  - Params:
    - offset: Offset to fetch, 0 by default
    - limit: How many networks to fetch, the service might return more in some cases, 10 by default, 50 maximum
  - Returns: A `types.PagedRes` object and a 200 if successful, a `types.APIError` object and a 400 or 500 if not (depending on the error)
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
  - Returns: An array of `types.BriefSeries` objects and a 200 if successful, a `types.APIError` object and a 500 if not
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
  - Returns: An array of `pointstores.Point` objects and a 200 if successful, a `types.APIError` object and a 404 or 500 if not (depending on the error)
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
