-module(pipeline_parallel_pattern_tests).
-include_lib("eunit/include/eunit.hrl").

%% Helper functions for the pipeline stages
add_one(X) -> X + 1.
multiply_by_two(X) -> X * 2.
subtract_three(X) -> X - 3.

%% Test case: Simple pipeline with parallel stages
simple_pipeline_test() ->
    %% Define the stages for the pipeline
    Stages = [
        fun add_one/1,
        fun multiply_by_two/1,
        fun subtract_three/1
    ],

    %% Run the pipeline with initial input 10
    {ok, Result} = pipeline_parallel_pattern:run_pipeline(Stages, 10),
    
    %% The expected result is ((10 + 1) * 2) - 3 = 19
    ?assertEqual(19, Result).

%% Test case: Pipeline with a single stage
single_stage_test() ->
    %% Define a single stage
    Stages = [fun add_one/1],

    %% Run the pipeline with initial input 10
    {ok, Result} = pipeline_parallel_pattern:run_pipeline(Stages, 10),
    
    %% The expected result is 10 + 1 = 11
    ?assertEqual(11, Result).

%% Test case: Empty pipeline (no stages)
empty_pipeline_test() ->
    %% No stages
    Stages = [],

    %% Run the pipeline with initial input 10
    {ok, Result} = pipeline_parallel_pattern:run_pipeline(Stages, 10),
    
    %% The expected result is the input itself
    ?assertEqual(10, Result).