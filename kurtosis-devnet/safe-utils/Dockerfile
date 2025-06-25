FROM node:22

RUN apt-get update && apt-get install -y python3 python3-venv python3-dev curl jq
RUN python3 -m venv /venv

ARG FOUNDRY_VERSION=1.1.0
RUN curl -L https://foundry.paradigm.xyz | bash
RUN /root/.foundry/bin/foundryup -i $FOUNDRY_VERSION

ARG SAFE_SINGLETON_VERSION=v1.0.43

#ARG SAFE_CONTRACT_VERSION=v1.4.1-3
ARG SAFE_CONTRACT_VERSION=main

ARG SAFE_CLI_VERSION=1.5.0

ARG SOLIDITY_VERSION=0.8.16 # needs to be aligned between contracts and singleton

ENV SOLIDITY_VERSION=$SOLIDITY_VERSION

WORKDIR /
RUN git clone --branch $SAFE_SINGLETON_VERSION https://github.com/safe-global/safe-singleton-factory
WORKDIR /safe-singleton-factory
RUN npm i

WORKDIR /
RUN git clone --branch $SAFE_CONTRACT_VERSION https://github.com/safe-global/safe-smart-account
# force use of the pinned singleton factory
RUN jq '.devDependencies["@safe-global/safe-singleton-factory"] = "file:///safe-singleton-factory"' safe-smart-account/package.json > package.json && mv package.json safe-smart-account/
WORKDIR /safe-smart-account
RUN npm i

WORKDIR /
RUN /venv/bin/pip install -U safe-cli==$SAFE_CLI_VERSION

ADD deploy_contracts.sh .
RUN chmod +x deploy_contracts.sh
ADD provision_wallets.sh .
RUN chmod +x provision_wallets.sh

ENTRYPOINT ["/bin/bash"]