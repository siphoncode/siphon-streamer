
siphon-streamer
===============

How to run the streamer on your local machine
---------------------------------------------

Install Go 1.5.1:

https://storage.googleapis.com/golang/go1.5.1.darwin-amd64.pkg

To run the Go server locally without docker:

    $ ./streamer

To run as a docker container within your local VirtualBox VM:

    $ ./run-local-containers.sh

Running tests
-------------

The tests are written in Python 3, so you will need a virtual environment:

    $ mkvirtualenv --python=`which python3` siphon-streamer
    $ workon siphon-streamer

Install the dependencies:

    $ cd /path/to/this/cloned/repo
    $ pip install -r test_requirements.txt

Run the tests:

    $ ./run-tests.sh

Deploying
---------

You should work on new features in a separate branch:

    $ git checkout -b my-new-feature

When you think it won't break, merge into the `staging` branch and the
orchestration server will deploy it for you:

    $ git checkout staging
    $ git merge my-new-feature
    $ git push origin staging
