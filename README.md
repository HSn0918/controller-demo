kubectl apply -f deploy.yaml
kubectl apply -f webhook.yaml
make install
make run
kubectl apply -f nginx.yaml
