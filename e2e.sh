#!/bin/bash

FUNC_TEST_ARGS=" -v " KUBEVIRT_E2E_FOCUS=SRIOV make functest
