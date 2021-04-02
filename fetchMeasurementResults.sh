#! /usr/bin/env bash

input=""
apiKey=""

# get input and apiKey from command line
while getopts f:a: opt; do
    case $opt in
        f)
            input=$OPTARG
            ;;
        a)
            apiKey=$OPTARG
            ;;
        \?)
            echo "Invalid option: -$OPTARG" >&2
            exit 1
            ;;
        :)
            echo "Option -$OPTARG requires an argument." >&2
            exit 1
            ;;
    esac
done
if [ ${#apiKey} == 0 ]; then
    echo "Need an API key to continue, use -a"
    exit 1
fi
if [ ${#input} == 0 ]; then
    echo "Need file to get measurement IDs, use -f"
    exit 1
fi
firstMeasurement=$(head -n 1 $input)
lastMeasurement=$(tail -n 1 $input)
folderName="$firstMeasurement-$lastMeasurement"
mkdir -p data/${folderName}
echo "Authorization: Key $apiKey"
echo "Writing measurement data to data/${folderName}/"
while read -r line; do
    echo "https://atlas.ripe.net/api/v2/measurements/$line/results/"
    curl -H "Authorization: Key $apiKey" "https://atlas.ripe.net/api/v2/measurements/$line/results/" > data/${folderName}/${line}_results.json
done < "$input"
echo "Wrote measurement data to data/${folderName}/"