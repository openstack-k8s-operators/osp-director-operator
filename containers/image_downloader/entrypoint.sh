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

# Source image URL without query args (if any)
RHEL_IMAGE_URL_STRIPPED=`echo $RHEL_IMAGE_URL | cut -f 1 -d \?`
if [[ $RHEL_IMAGE_URL_STRIPPED =~ qcow2(.[gx]z)?$ ]]; then
    # Source image filename
    RHEL_IMAGE_FILENAME_RAW=$(basename $RHEL_IMAGE_URL_STRIPPED)
    # Source image filename without compressed file extension (if any compressed file extension)
    RHEL_IMAGE_FILENAME_OPENSTACK=${RHEL_IMAGE_FILENAME_RAW/.[gx]z}
    # Source image filename compression extension (if any)
    IMAGE_FILENAME_EXTENSION=${RHEL_IMAGE_FILENAME_RAW/$RHEL_IMAGE_FILENAME_OPENSTACK}
    # Remote path to source image, minus filename
    IMAGE_URL=$(dirname $RHEL_IMAGE_URL_STRIPPED)
else
    echo "Unexpected image format $RHEL_IMAGE_URL"
    exit 1
fi

# Local filename under which we'll store the image
RHEL_IMAGE_FILENAME_COMPRESSED="compressed-${RHEL_IMAGE_FILENAME_OPENSTACK}"
FFILENAME="rhel-latest.qcow2"

# Temporary directory to hold the downloaded source image
mkdir -p /usr/local/apache2/htdocs/images /usr/local/apache2/htdocs/tmp
TMPDIR=$(mktemp -d -p /usr/local/apache2/htdocs/tmp)
cd $TMPDIR

# We have a File in the cache that matches the one we want, use it
if [ -s "/usr/local/apache2/htdocs/images/$RHEL_IMAGE_FILENAME_OPENSTACK/$RHEL_IMAGE_FILENAME_COMPRESSED.md5sum" ]; then
    echo "$RHEL_IMAGE_FILENAME_OPENSTACK/$RHEL_IMAGE_FILENAME_COMPRESSED.md5sum found, contents:"
    cat /usr/local/apache2/htdocs/images/$RHEL_IMAGE_FILENAME_OPENSTACK/$RHEL_IMAGE_FILENAME_COMPRESSED.md5sum
else
    # Source image not found locally, so download it and then compress it
    CONNECT_TIMEOUT=120
    MAX_ATTEMPTS=5
    DOWNLOADED=0
    COMPRESSED_FLAG=""

    if [ -n "$IMAGE_FILENAME_EXTENSION" ]; then
      COMPRESSED_FLAG=" --compressed"
    fi

    # Download the actual remote file to our local temporary directory
    for i in $(seq ${MAX_ATTEMPTS}); do
        if ! curl -g --insecure${COMPRESSED_FLAG} -L --connect-timeout ${CONNECT_TIMEOUT} -o "${RHEL_IMAGE_FILENAME_RAW}" "${IMAGE_URL}/${RHEL_IMAGE_FILENAME_RAW}"; then
          SLEEP_TIME=$((i*i))
          echo "Download failed, retrying after ${SLEEP_TIME} seconds..."; 
          sleep ${SLEEP_TIME}
        else
          DOWNLOADED=1
          break
        fi
    done

    if [ $DOWNLOADED -ne 1 ]; then
      echo "Download failed for ${IMAGE_URL}/${RHEL_IMAGE_FILENAME_RAW}"
      exit 1
    fi

    # If the source file is compressed, unzip it
    if [[ $IMAGE_FILENAME_EXTENSION == .gz ]]; then
      gzip -d "$RHEL_IMAGE_FILENAME_RAW"
    elif [[ $IMAGE_FILENAME_EXTENSION == .xz ]]; then
      unxz "$RHEL_IMAGE_FILENAME_RAW"
    fi

    # Turn source file image into local qcow2 file image, compressed, with name "RHEL_IMAGE_FILENAME_COMPRESSED",
    # and calculate its md5sum (and store in temporary dir, /usr/local/apache2/htdocs/tmp)
    qemu-img convert -O qcow2 -c "$RHEL_IMAGE_FILENAME_OPENSTACK" "$RHEL_IMAGE_FILENAME_COMPRESSED"
    md5sum "$RHEL_IMAGE_FILENAME_COMPRESSED" | cut -f 1 -d " " > "$RHEL_IMAGE_FILENAME_COMPRESSED.md5sum"
fi

# Now move the locally-created and compressed qcow2 image from the temporary directory (/usr/local/apache2/htdocs/tmp)
# into the actual Apache webserver directory (/usr/local/apache2/htdocs/images), if necessary
if [ -s "${RHEL_IMAGE_FILENAME_COMPRESSED}.md5sum" ] ; then
    cd /usr/local/apache2/htdocs/images
    chmod 755 $TMPDIR
    mv $TMPDIR $RHEL_IMAGE_FILENAME_OPENSTACK
    # TODO: If we change this container from an init to a side-car, we will need to get rid
    # of this linking or otherwise somehow change it, as the side-car version will want to 
    # store multiple images in the same Apache server
    ln -sf "$RHEL_IMAGE_FILENAME_OPENSTACK/$RHEL_IMAGE_FILENAME_COMPRESSED" $FFILENAME
    ln -sf "$RHEL_IMAGE_FILENAME_OPENSTACK/$RHEL_IMAGE_FILENAME_COMPRESSED.md5sum" "$FFILENAME.md5sum"
else
    # Nothing new was downloaded (because the ${RHEL_IMAGE_FILENAME_COMPRESSED}.md5sum file is not
    # present in the temp directory), so just delete the temp directory
    rm -rf $TMPDIR
fi
