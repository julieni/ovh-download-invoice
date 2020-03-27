# OVH download invoice

## help

run `./ovh-download-invoice -h`

## first use - setup

run `./ovh-download-invoice init`

setup your own API credentials and note them down

you can report them in your .env file (copied from .env.example file)

## download invoices for current month

run `./ovh-download-invoice download`

## download invoices for given month and/or year

run `./ovh-download-invoice download --year 2019 --month 03`