%%%-------------------------------------------------------------------
%% @doc dpe_bindings public API
%% @end
%%%-------------------------------------------------------------------

-module(dpe_bindings_app).

-behaviour(application).

-export([start/2, stop/1]).

start(_StartType, _StartArgs) ->
    dpe_bindings_sup:start_link().

stop(_State) ->
    ok.

%% internal functions
