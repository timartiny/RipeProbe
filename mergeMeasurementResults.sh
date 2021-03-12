#! /usr/bin/env bash

measurementFile=""
lookupFile=""
# get file that contiains measurements and lookup
while getopts m:l: opt; do
    case $opt in
        m)
            measurementFile=$OPTARG
            ;;
        l)
            lookupFile=$OPTARG
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
if [ ${#measurementFile} == 0 ]; then
    echo "Need a file containing measurement ids, use -m"
    exit 1
fi
if [ ${#lookupFile} == 0 ]; then
    echo "Need file lookup file to add results to, use -l"
    exit 1
fi
while read -r line; do
    ./parseresults -f $lookupFile -id $line
done < "$measurementFile"