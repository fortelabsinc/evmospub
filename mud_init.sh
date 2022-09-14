#!/bin/sh

PATH_HOME="$HOME/.evmosd"
PATH_CFG="$PATH_HOME/config"
KEY_RING="test"
KEY_NAME="genesis"
KEY_MNEMONIC="acid orphan sting arm attract shallow tiny begin never patient ethics elbow stove middle lady honey undo news observe problem number frozen gasp leg"
CHAIN_LOGLEVEL="info"
CHAIN_MONIKER="mudblocks"
CHAIN_ID="mudblocks_9000-1"
CHAIN_DENOM="mud"
CHAIN_MINGAS="0"
CHAIN_MAXGAS="10000000"
CHAIN_BLOCKTIMEMS="30000"
CHAIN_GENSUPPLY="100000000000000000000000000"
CHAIN_VALIDATORSUPPLY="1000000000000000000000"
CHAIN_CLAIMAMOUNT="10000"
CHAIN_TRACE=""
CHAIN_PRUNING="nothing"
EVMOS_RELEASE="v6.0.2"
KEY_ALGO="eth_secp256k1"
CHAIN_JRPCAPI="eth,txpool,personal,net,debug,web3"


rm -rf $PATH_HOME/

# Init config
evmosd config chain-id $CHAIN_ID && \
evmosd config keyring-backend $KEY_RING && \
evmosd init $CHAIN_MONIKER --chain-id $CHAIN_ID 

# Add keys
echo $KEY_MNEMONIC | evmosd keys add $KEY_NAME --keyring-backend $KEY_RING --algo $KEY_ALGO --recover

TMP="$PATH_CFG/tmp_genesis.json"
GEN="$PATH_CFG/genesis.json"

# Change token denom
cat $GEN | jq -r --arg CHAIN_DENOM "$CHAIN_DENOM" '.app_state["staking"]["params"]["bond_denom"]=$CHAIN_DENOM' > $TMP && mv $TMP $GEN && \
cat $GEN | jq -r --arg CHAIN_DENOM "$CHAIN_DENOM" '.app_state["crisis"]["constant_fee"]["denom"]=$CHAIN_DENOM' > $TMP && mv $TMP $GEN && \
cat $GEN | jq -r --arg CHAIN_DENOM "$CHAIN_DENOM" '.app_state["claims"]["params"]["claims_denom"]=$CHAIN_DENOM' > $TMP && mv $TMP $GEN && \
cat $GEN | jq -r --arg CHAIN_DENOM "$CHAIN_DENOM" '.app_state["gov"]["deposit_params"]["min_deposit"][0]["denom"]=$CHAIN_DENOM' > $TMP && mv $TMP $GEN && \
cat $GEN | jq -r --arg CHAIN_DENOM "$CHAIN_DENOM" '.app_state["inflation"]["params"]["mint_denom"]=$CHAIN_DENOM' > $TMP && mv $TMP $GEN && \
cat $GEN | jq -r --arg CHAIN_DENOM "$CHAIN_DENOM" '.app_state["mint"]["params"]["mint_denom"]=$CHAIN_DENOM' > $TMP && mv $TMP $GEN && \
cat $GEN | jq -r --arg CHAIN_DENOM "$CHAIN_DENOM" '.app_state["evm"]["params"]["evm_denom"]=$CHAIN_DENOM' > $TMP && mv $TMP $GEN

# Set gas limit
cat $GEN | jq -r --arg CHAIN_MAXGAS "$CHAIN_MAXGAS" '.consensus_params["block"]["max_gas"]=$CHAIN_MAXGAS' > $TMP && mv $TMP $GEN

# Set claims decay (2 entries not present)
NODE_ADDRESS=$(evmosd keys list | grep  "address: " | cut -c12-) && \
CURRENT_DATE=$(date -u +"%Y-%m-%dT%TZ") && \
cat $GEN | jq -r --arg CURRENT_DATE "$CURRENT_DATE" '.app_state["claims"]["params"]["airdrop_start_time"]=$CURRENT_DATE' > $TMP && mv $TMP $GEN && \
cat $GEN | jq -r --arg NODE_ADDRESS "$NODE_ADDRESS" --arg CHAIN_CLAIMAMOUNT "$CHAIN_CLAIMAMOUNT" '.app_state["claims"]["claims_records"]=[{"initial_claimable_amount":$CHAIN_CLAIMAMOUNT, "actions_completed":[false, false, false, false],"address":$NODE_ADDRESS}]' > $TMP && mv $TMP $GEN && \
# cat $GEN | jq -r '.app_state["claim"]["params"]["duration_of_decay"]="1000000s"' > $TMP && mv $TMP $GEN && \ 
# cat $GEN | jq -r '.app_state["claim"]["params"]["duration_until_decay"]="100000s"' > $TMP && mv $TMP $GEN && \ 
cat $GEN | jq -r --arg CHAIN_DENOM "$CHAIN_DENOM" --arg CHAIN_CLAIMAMOUNT "$CHAIN_CLAIMAMOUNT" '.app_state["bank"]["balances"] += [{"address":"evmos15cvq3ljql6utxseh0zau9m8ve2j8erz89m5wkz","coins":[{"denom":$CHAIN_DENOM, "amount":$CHAIN_CLAIMAMOUNT}]}]' > $TMP && mv $TMP $GEN

# Disable empty block generation
CFGTOML="$PATH_CFG/config.toml"
sed -i 's/create_empty_blocks = true/create_empty_blocks = false/g' $CFGTOML

# Remove seeds
sed -i 's/seeds = \".*\"/seeds = \"\"/g' $CFGTOML

# Setup genesis account
evmosd add-genesis-account $KEY_NAME "${CHAIN_GENSUPPLY}${CHAIN_DENOM}" --keyring-backend $KEY_RING

# Update total supply (genesis, validator)
TOTAL_SUPPLY=$(printf '%s + %s\n' "$CHAIN_GENSUPPLY" "$CHAIN_CLAIMAMOUNT" | bc) && \
cat $GEN | jq -r --arg TOTAL_SUPPLY "$TOTAL_SUPPLY" '.app_state["bank"]["supply"][0]["amount"]=$TOTAL_SUPPLY' > $TMP && mv $TMP $GEN

# Setup validator account
evmosd gentx $KEY_NAME "${CHAIN_VALIDATORSUPPLY}${CHAIN_DENOM}" --keyring-backend $KEY_RING --chain-id $CHAIN_ID && \
evmosd collect-gentxs

# Validate genesis file
evmosd validate-genesis

# Created account "alice" with address "evmos1ckg040r6tnjfzfwm9skj67qhyqwh3qp4cy5qgn" with mnemonic: "caution also galaxy match upset cheap slow aisle alley credit place share run shoe oxygen pole arrow sauce clip plunge defy absorb car concert"
echo "caution also galaxy match upset cheap slow aisle alley credit place share run shoe oxygen pole arrow sauce clip plunge defy absorb car concert" | evmosd keys add alice --recover

# Created account "bob" with address "evmos1qy47vryjvtu0exwp8q0ufelgper6ag4kxud4h0" with mnemonic: "endless suspect clump job wagon control wonder project leave dream vendor inform cry tobacco lab youth prison cereal absurd bulb toy student tissue cabbage"
echo "endless suspect clump job wagon control wonder project leave dream vendor inform cry tobacco lab youth prison cereal absurd bulb toy student tissue cabbage" | evmosd keys add bob --recover

# Enable swagger
sed -i 's/swagger = false/swagger = true/g' $PATH_HOME/config/app.toml && \
cat $PATH_HOME/config/app.toml | grep swagger

# Enables the api
sed -i '/\[api\]/,/enable = false/s/enable = false/enable = true/' $PATH_HOME/config/app.toml

# Disable CORS
sed -i 's/enable-unsafe-cors = false/enable-unsafe-cors = true/g' $PATH_HOME/config/app.toml && \
cat $PATH_HOME/config/app.toml | grep cors
sed -i 's/enabled-unsafe-cors = false/enabled-unsafe-cors = true/g' $PATH_HOME/config/app.toml && \
cat $PATH_HOME/config/app.toml | grep cors
sed -i 's/cors_allowed_origins = \[\]/cors_allowed_origins = \["*"\]/g' $PATH_HOME/config/config.toml && \
cat $PATH_HOME/config/config.toml | grep cors

CHAIN_MINGASPRICE="${CHAIN_MINGAS}${CHAIN_DENOM}"

evmosd start $CHAIN_TRACE \
  --rpc.laddr "tcp://0.0.0.0:26659" \
  --rpc.pprof_laddr "127.0.0.1:6061" \
  --p2p.laddr "0.0.0.0:26658" \
  --grpc.address "0.0.0.0:9092" \
  --grpc-web.address "0.0.0.0:9093" \
  --pruning "$CHAIN_PRUNING" \
  --log_level "$CHAIN_LOGLEVEL" \
  --minimum-gas-prices "$CHAIN_MINGASPRICE" \
  --fees "0$CHAIN_DENOM" \
  --json-rpc.api="$CHAIN_JRPCAPI" \
  --json-rpc.address "0.0.0.0:8547" \
  --json-rpc.ws-address "0.0.0.0:8548"
