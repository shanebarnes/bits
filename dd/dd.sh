#!/bin/bash

set -e

args=( ${@} )
arg_of_value=
arg_of_index=
file_count=0
file_limit=100
proc_count=
os_name=$(uname -s)
os_name=$(echo "$os_name" | awk '{print tolower($0)}') # bash 4: os_name=${os_name,,}

function get_proc_count() {
    case ${os_name} in
        "darwin")
            proc_count=$(sysctl -n hw.ncpu) ;;
        "linux")
            proc_count=$(nproc) ;;
        *)
            proc_count=1 ;;
    esac
}

get_proc_count

for ((i=0; i<${#args[@]}; i++));
do
    if [[ ${args[${i}]} == of* ]]; then
        arg_of_value=${args[${i}]}
        arg_of_index=${i}
    fi
done

printf "Starting %d workers\n" ${proc_count}

start_sec=$(date +%s)

while [ ${file_count} -lt ${file_limit} ]; do
    job_count=$((file_limit-file_count))
    if [ ${job_count} -gt ${proc_count} ]; then
        job_count=${proc_count}
    fi

    for ((j=0; j<job_count; j++)); do
        file_count=$((file_count+1))
        args[${arg_of_index}]="${arg_of_value}.${file_count}"
        dd ${args[@]} >/dev/null 2>&1 &
    done

    wait $(jobs -p)

    progress=$((file_count*100/file_limit))
    stop_sec=$(date +%s)
    elapsed_sec=$((stop_sec-start_sec))
    printf "seconds: %-6d files: %6d/%-6d progress: %3d%%\r" ${elapsed_sec} ${file_count} ${file_limit} ${progress}
done

printf "\n"

exit 0
