# Setup

## setup etcd data

> set key /apps/v1/user/e7ce2504-d41c-4657-992b-a1d02c11bb33 with data: user.json

## generate token

```shell
apiserver --secret=xxx --user=e7ce2504-d41c-4657-992b-a1d02c11bb33 token
```

## run server

```shell
# 1
apiserver --secret=xxx

# 2
scheduler --token=xxxxx

# 3
controller-manager --token=xxxxx

#4
worker --token=xxxxx --worker-id=1
worker --token=xxxxx --worker-id=2
```

## run web ui

> see https://github.com/tsundata/flowline-admin
