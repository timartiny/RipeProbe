#! /usr/bin/env bash

measurementFile=""
# get file that contiains measurements and lookup
while getopts m: opt; do
    case $opt in
        m)
            measurementFile=$OPTARG
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
while read -r line; do
    ./whiteboardresults -id $line
done < "$measurementFile"