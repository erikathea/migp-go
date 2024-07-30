#!/bin/bash
source ~/myvenv/bin/activate
cd ~/src/PagPassGPT/
python generate_pw_variant.py --input_password="$1" --generate_num="$2" --compute_loglikelihood
deactivate
