#!/bin/bash -xe

# Check and set http(s)_proxy. Required for cURL to use a proxy
export http_proxy=${http_proxy:-$HTTP_PROXY}
export https_proxy=${https_proxy:-$HTTPS_PROXY}

# Which image should we use
export RHEL_IMAGE_URL=${1:-$RHEL_IMAGE_URL}
if [ -z "$RHEL_IMAGE_URL" ] ; then
    echo "No image URL provided"
    exit 1
fi

RHEL_IMAGE_URL_STRIPPED=`echo $RHEL_IMAGE_URL | cut -f 1 -d \?`
if [[ $RHEL_IMAGE_URL_STRIPPED =~ qcow2(.[gx]z)?$ ]]; then
    RHEL_IMAGE_FILENAME_RAW=$(basename $RHEL_IMAGE_URL_STRIPPED)
    RHEL_IMAGE_FILENAME_OPENSTACK=${RHEL_IMAGE_FILENAME_RAW/.[gx]z}
    IMAGE_FILENAME_EXTENSION=${RHEL_IMAGE_FILENAME_RAW/$RHEL_IMAGE_FILENAME_OPENSTACK}
    IMAGE_URL=$(dirname $RHEL_IMAGE_URL_STRIPPED)
else
    echo "Unexpected image format $RHEL_IMAGE_URL"
    exit 1
fi

RHEL_IMAGE_FILENAME_COMPRESSED="compressed-${RHEL_IMAGE_FILENAME_OPENSTACK}"
FFILENAME="rhel-latest.qcow2"

mkdir -p /usr/local/apache2/htdocs/images /usr/local/apache2/htdocs/tmp
TMPDIR=$(mktemp -d -p /usr/local/apache2/htdocs/tmp)
cd $TMPDIR

# We have a File in the cache that matches the one we want, use it
if [ -s "/usr/local/apache2/htdocs/images/$RHEL_IMAGE_FILENAME_OPENSTACK/$RHEL_IMAGE_FILENAME_COMPRESSED.md5sum" ]; then
    echo "$RHEL_IMAGE_FILENAME_OPENSTACK/$RHEL_IMAGE_FILENAME_COMPRESSED.md5sum found, contents:"
    cat /usr/local/apache2/htdocs/images/$RHEL_IMAGE_FILENAME_OPENSTACK/$RHEL_IMAGE_FILENAME_COMPRESSED.md5sum
else
    CONNECT_TIMEOUT=120
    MAX_ATTEMPTS=5

    for i in $(seq ${MAX_ATTEMPTS}); do
        if ! curl -g --insecure --compressed -L --connect-timeout ${CONNECT_TIMEOUT} -o "${RHEL_IMAGE_FILENAME_RAW}" "${IMAGE_URL}/${RHEL_IMAGE_FILENAME_RAW}"; then
          SLEEP_TIME=$((i*i))
          echo "Download failed, retrying after ${SLEEP_TIME} seconds..."; 
          sleep ${SLEEP_TIME}
        else
          break
        fi
    done

    if [[ $IMAGE_FILENAME_EXTENSION == .gz ]]; then
      gzip -d "$RHEL_IMAGE_FILENAME_RAW"
    elif [[ $IMAGE_FILENAME_EXTENSION == .xz ]]; then
      unxz "$RHEL_IMAGE_FILENAME_RAW"
    fi

    qemu-img convert -O qcow2 -c "$RHEL_IMAGE_FILENAME_OPENSTACK" "$RHEL_IMAGE_FILENAME_COMPRESSED"
    md5sum "$RHEL_IMAGE_FILENAME_COMPRESSED" | cut -f 1 -d " " > "$RHEL_IMAGE_FILENAME_COMPRESSED.md5sum"
fi

if [ -s "${RHEL_IMAGE_FILENAME_COMPRESSED}.md5sum" ] ; then
    cd /usr/local/apache2/htdocs/images
    chmod 755 $TMPDIR
    mv $TMPDIR $RHEL_IMAGE_FILENAME_OPENSTACK
    ln -sf "$RHEL_IMAGE_FILENAME_OPENSTACK/$RHEL_IMAGE_FILENAME_COMPRESSED" $FFILENAME
    ln -sf "$RHEL_IMAGE_FILENAME_OPENSTACK/$RHEL_IMAGE_FILENAME_COMPRESSED.md5sum" "$FFILENAME.md5sum"
else
    rm -rf $TMPDIR
fi
