#/usr/bin/env bash 

# a flag used to tell if the build failed
fails=""

# Invoke gofmt on the current directory.
# -l flag causes it to only list files
GOFMT_RESULTS=`gofmt -l .`
if [ "$GOFMT_RESULTS" ]; then
	echo "FAIL: UNFORMATTED FILES:"
	echo "GOFMT FINDS"
	echo "$GOFMT_RESULTS"
	fails="fails"
fi

#Lastly, make sure calling ./query/build.sh doesn't cause ./query/language.peg.go to change

before=$(cat ./query/language.peg.go)
./query/build.sh
# we have to reformat in case peg.go produces unformatted code
gofmt -w ./query/language.peg.go
after=$(cat ./query/language.peg.go)

if [ "$before" != "$after" ]; then
	echo "FAIL: LANGUAGE .GO FILE IS NOT UP TO DATE"
	echo "THERE WERE CHANGES TO query/language.peg AND NO CHANGES TO query/language.peg.go"
	echo "Make sure you ran the build file, and that your version of peg is up to date."
	echo "To get the latest version of peg, run:"
	echo "> go get -u github.com/pointlander/peg"
	fails="fails"
fi

if [ $fails ]
then
	exit -1
fi


