#!/bin/bash

# TODO Make this generic for both toml and lock file
function removeDependency()
{
    local dependencyString # Input 1. Ex "github.com/statoil/radix-operator"
    local cutFromString # Input 2. Ex "[[projects]]""
    local targetFilepath # Input 2. Ex "./Gopkg.lock"    
    local lineNumOfDependency
    local cutFrom
    local textToCut
    local cutTo

    dependencyString="$1"
    cutFromString="$2"
    targetFilepath="$3"

    lineNumOfDependency="$(grep -n $dependencyString $targetFilepath | head -n 1 | cut -d: -f1)"
    cutFrom="$(expr $lineNumOfDependency - 2)"
    lineNumbersToCut="$(gawk '{ print NR, $1 }' $targetFilepath | gawk 'NR=='$cutFrom',/[[projects]]/' | cut -d ' ' -f1)"
    cutTo="$(echo $lineNumbersToCut | gawk '{print $NF}')"
    sed -ri "$cutFrom,${cutTo}d" $targetFilepath
}

removeDependency "$@"
