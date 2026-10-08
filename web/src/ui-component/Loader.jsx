// ==============================|| LOADER ||============================== //
const Loader = () => (
  <div className="fixed left-0 top-0 z-[1301] w-full overflow-hidden">
    <style>{'@keyframes loader-indeterminate{0%{margin-left:-40%;width:40%}50%{margin-left:30%;width:50%}100%{margin-left:100%;width:40%}}'}</style>
    <div className="h-1 bg-primary animate-[loader-indeterminate_1.1s_ease-in-out_infinite]" />
  </div>
);

export default Loader;
