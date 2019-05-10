#!/bin/bash
chmod 700 /root/.ssh
ssh-keyscan -H github.com >> "/root/.ssh/known_hosts"
chmod 400 "/root/.ssh/known_hosts"
chmod 400 /root/.ssh/id_rsa
