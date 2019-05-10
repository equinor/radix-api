#!/bin/bash
# chmod 700 /root/.ssh
# chmod 400 /root/.ssh/id_rsa

SHA256_RSA_FINGERPRINT="SHA256:nThbg6kXUpJWGl7E1IGOCspRomTxdCARLviKw6E5SY8"
KNOWN_HOSTS=/root/dynamic_known_host
# ssh-keyscan github.com 2>/dev/null | tee keyscan.txt | cut -f 2,3 -d ' ' | ssh-keygen -lf - | cut -f 2 -d ' ' | grep -q "$SHA256_RSA_FINGERPRINT" && cat keyscan.txt >> $KNOWN_HOSTS && rm keyscan.txt || (echo FAIL && exit 1)
ssh-keyscan -H github.com >> $KNOWN_HOSTS

chmod 400 $KNOWN_HOSTS
/bin/sh -c bash
