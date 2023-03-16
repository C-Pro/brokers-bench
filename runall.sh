#!/bin/bash

if [ "$1" == "sync" ]
then
    . tests/5producers
    for ip in ${IPS[@]}
    do
        ssh -i pk.pk ubuntu@$ip sudo apt install -y chrony
        ssh -i pk.pk ubuntu@$ip 'echo "server 169.254.169.123 prefer iburst minpoll 4 maxpoll 4" > chrony.conf'
        ssh -i pk.pk ubuntu@$ip 'sudo cat /etc/chrony/chrony.conf >> chrony.conf'
        ssh -i pk.pk ubuntu@$ip 'cat chrony.conf | sudo tee /etc/chrony/chrony.conf'
        ssh -i pk.pk ubuntu@$ip sudo /etc/init.d/chrony restart
        ssh -i pk.pk ubuntu@$ip sudo chronyd -q
    done
    exit 0
fi

if [ "$1" == "kill" ]
then
    . tests/5producers
    for ip in ${IPS[@]}
    do
        ssh -i pk.pk ubuntu@$ip killall bench
        ssh -i pk.pk ubuntu@$ip 'ps -ef | grep bench'
    done
    exit 0
fi


if [ "$1" == "copy" ]
then
    . tests/5producers
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bench .

    for ip in ${IPS[@]}
    do
        scp -i pk.pk ./bench ./run.sh ubuntu@$ip:
    done
    exit 0
fi

TEST_NAME="${1:-5producers}"

# Load test settings.
. tests/$TEST_NAME

echo "Running $TEST_NAME"

# Start benchmarks on all instances.
for ((i=0;i<${#IPS[@]};i++))
do
    echo "Starting ./run.sh ${TOPICS[$i]} at ${IPS[$i]}"
    ssh -i pk.pk ubuntu@${IPS[$i]} ./run.sh \"$PARAMS -topics=${TOPICS[$i]}\"
done

# 5 min to run all benchmarks.
sleep 315

mkdir $TEST_NAME
pushd $TEST_NAME

# truncate files
>latencies.csv
>stdout.txt
for ip in ${IPS[@]}
do
    echo "copying latencies.csv from $ip"
    ssh -i ../pk.pk ubuntu@$ip cat latencies.csv >> latencies.csv
    echo "copying bench.out from $ip"
    ssh -i ../pk.pk ubuntu@$ip cat bench.out >> stdout.txt
done

cat stdout.txt

popd
