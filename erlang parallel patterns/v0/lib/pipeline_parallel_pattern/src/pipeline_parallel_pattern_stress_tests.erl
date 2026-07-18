-module(pipeline_parallel_pattern_stress_tests).
-export([stress_test_large_input/0]).
-include_lib("eunit/include/eunit.hrl").

%% Helper functions for the pipeline stages
add_one(X) -> X + 1.
multiply_by_two(X) -> X * 2.
subtract_three(X) -> X - 3.
square(X) -> X * X.
factorial(N) -> lists:foldl(fun(X, Acc) -> X * Acc end, 1, lists:seq(1, N)).

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

%% Test case: Complex pipeline with larger data and more stages
complex_pipeline_test() ->
    %% Define more complex stages
    Stages = [
        fun add_one/1,
        fun multiply_by_two/1,
        fun subtract_three/1,
        fun square/1,
        fun factorial/1
    ],

    %% Run the pipeline with initial input 5
    {ok, Result} = pipeline_parallel_pattern:run_pipeline(Stages, 5),
    
    %% Expected calculation: 
    %% Start with 5
    %% (5 + 1) = 6
    %% (6 * 2) = 12
    %% (12 - 3) = 9
    %% (9 * 9) = 81
    %% Factorial of 81 is a very large number, so we'll precompute it
    ExpectedResult = factorial(81),
    ?assertEqual(ExpectedResult, Result).

%% Test case: Stress test with a large input list
stress_test_large_input() ->
    %% Create a large input list
    LargeInput = lists:seq(1, 1000000),

    %% Define simple stages that work on lists
    Stages = [
        fun(L) -> [X + 1 || X <- L] end,
        fun(L) -> [X * 2 || X <- L] end,
        fun(L) -> [X - 3 || X <- L] end
    ],

    %% Run the pipeline with the large input list
    {ok, Result} = pipeline_parallel_pattern:run_pipeline(Stages, LargeInput),

    %% Verify the size of the result is the same as the input list
    ?assertEqual(length(LargeInput), length(Result)),

    %% Check a sample value to ensure correctness
    %% The first value should be ((1 + 1) * 2) - 3 = 1
    %% The last value should be ((10000 + 1) * 2) - 3 = 20001
    ?assertEqual(1, hd(Result)),
    ?assertEqual(20001, lists:last(Result)).

%% Test case: Extremely large factorial calculation
extreme_factorial_test() ->
    %% Define a stage that calculates factorial of a large number
    Stages = [fun factorial/1],

    %% Run the pipeline with initial input 20
    {ok, Result} = pipeline_parallel_pattern:run_pipeline(Stages, 20),
    
    %% The expected result is factorial of 20 = 2,432,902,008,176,640,000
    ?assertEqual(2432902008176640000, Result).
