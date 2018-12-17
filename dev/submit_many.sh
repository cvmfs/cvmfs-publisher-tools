#!/bin/sh

staging_server=$1
# Submit a set of independent jobs
ids=""
for i in $(seq 1 10) ; do
    id=$(./conveyor submit \
         --repo test-sw.hsf.org \
         --payload ${staging_server}/ripgrep-0.$i.0-x86_64-unknown-linux-musl.tar.gz \
         --path /ripgrep-0.$i.0 | tail -1 | jq -r .ID)
    ids="$ids $id"
done
ids=$(echo $ids | tr ' ' ,)

# Submit a final job depending on all the previous ones
./conveyor submit --repo test-sw.hsf.org --deps "$ids" --script /usr/local/bin/list_all_versions.sh --wait
