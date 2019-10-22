mkdir -p ./built
nexe ./index.js -t ./mac-x64-12.9.1
mv ./credential-broker ./built/broker-mac-x64
nexe ./index.js -t ./linux-x64-12.9.1
mv ./credential-broker ./built/broker-linux-x64
nexe ./index.js -t ./windows-x64-12.9.1
mv ./credential-broker ./built/broker-windows-x64
