# What's this ?
This is an HTTP log parser. It reads log lines in Common Log Format (CLF)
and prints some useful statistics (top Hits, bandwidth)
on stdout at regular intervals. It also print alerts on the console whenever
certain thresholds are crossed.

# Build
### Without Docker
```console
go build .
```
### With Docker
```console
docker build -t monitor .
```

# Run
### Without Docker
```console
./monitor -logpath /var/log/nginx/access.log -alertThreshold 10
```
### With Docker
```console
docker run monitor:latest -logpath /tmp/access.log
```

# Test
```console
go test -v ./...
```

# Possible improvements on the overall solution
* Use a real timeseries DB. Here I implemented an in-memory
"data container" that is probably much slower and less efficient than
a full blown TSDB. Beside, in-mem has no data persistance :) But I
felt that using a 3rd party DB was cheating.
* Though the code is structured as a pipeline, with each step executing
in a goroutine, some steps could be parallelized (the `readlines` and
`parselines` for instance), each executed in several goroutines.
* The aggregation step is a bottleneck. We could shard the aggregation
with one aggregator per `sectionName` but with need a "router" that routes
the HTTP hit to a specific aggregator. Leveraging `Kafka partitions` and
partitioning using the key `sectionName` would work. 
* The current design requires that logs are written to the log file
in chronological order. That is, if line `A` is written before line `B`,
then `A` must represent an HTTP hit that happened before the one 
corresponding to `B`. Kafka's semantic can help here too.
