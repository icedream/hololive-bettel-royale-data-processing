import { ChangeEventHandler, KeyboardEventHandler } from 'react';
import { Form, Icon, Loader } from 'react-bulma-components';
import { Duration, MaybeTimeDuration } from 'typed-duration'
import { debounce } from '../../utils/debounce';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome'
import { faSearch } from '@fortawesome/free-solid-svg-icons';

const { milliseconds } = Duration

type FormInput = typeof Form.Input;

export interface SearchFieldProps extends React.ComponentProps<FormInput> {
  onChange?: ChangeEventHandler<HTMLInputElement>,
  delay: MaybeTimeDuration,
  working: boolean,
};

const defaultDebounceDuration = milliseconds.from(250);

function SearchField({
  onChange,
  delay,
  working,
  ...props
}: SearchFieldProps) {

  var realOnChange: ChangeEventHandler<HTMLInputElement> | undefined;
  var realOnKeyDown: KeyboardEventHandler<HTMLInputElement> | undefined;
  if (onChange) {
    const [debouncedOnChange, teardownOnChange] = debounce(onChange, delay);
    realOnChange = (e) => debouncedOnChange(e);
    realOnKeyDown = (e) => {
      // allow user to abort execution altogether with escape
      if (e.key === 'Escape') {
        teardownOnChange();
      }
      if (props.onKeyDown) {
        props.onKeyDown(e);
      }
    }
  }

  return (
    <Form.Control>
      <Form.Input
        onChange={realOnChange}
        onKeyDown={realOnKeyDown}
        {...props} />
      <Icon align="left">
        {
          working
            ? (
              <Loader />
            )
            : (
              <FontAwesomeIcon icon={faSearch} />
            )
        }
      </Icon>
    </Form.Control >
  )
}

SearchField.defaultProps = {
  delay: defaultDebounceDuration,
  working: false,
};

export default SearchField;
