mkdir -p ./built/mac
nexe ./index.js -t ./mac-x64-12.9.1
mv ./credential-broker ./built/mac/broker
mkdir -p ./built/linux
nexe ./index.js -t ./linux-x64-12.9.1
mv ./credential-broker ./built/linux/broker
mkdir -p ./built/windows
nexe ./index.js -t ./windows-x64-12.9.1
mv ./credential-broker ./built/windows/broker
